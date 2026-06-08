// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package inplaceupdate

import (
	"context"
	"fmt"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	// podEvictionRetryInterval is the time to wait before retrying eviction of PDB-protected pods.
	// Matches MCM's PodEvictionRetryInterval.
	podEvictionRetryInterval = 20 * time.Second
	// drainTimeout is the overall maximum duration for a drain operation before force-deleting pods.
	drainTimeout = 20 * time.Minute
	// updateTimeout is the maximum duration to wait for GNA to complete the in-place update after
	// the node has been drained. If exceeded, the update is marked as failed.
	updateTimeout = 30 * time.Minute
)

// Reconciler orchestrates in-place updates for all nodes in a worker pool.
type Reconciler struct {
	SeedClient            client.Client
	Clock                 clock.Clock
	Workers               []gardencorev1beta1.Worker
	ControlPlaneNamespace string
}

// Reconcile processes all in-place-update state for a single worker pool.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)
	poolSecretName := req.Name

	nodeList := &corev1.NodeList{}
	if err := r.SeedClient.List(ctx, nodeList, client.MatchingLabels{
		v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: poolSecretName,
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list nodes for pool %s: %w", poolSecretName, err)
	}
	if len(nodeList.Items) == 0 {
		return reconcile.Result{}, nil
	}

	poolName := nodeList.Items[0].Labels[v1beta1constants.LabelWorkerPool]
	maxUnavailable := r.MaxUnavailableForPool(poolName, len(nodeList.Items))

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		switch {
		case node.Labels[machinev1alpha1.LabelKeyNodeUpdateResult] == machinev1alpha1.LabelValueNodeUpdateSuccessful:
			if err := r.cleanupAfterSuccessfulUpdate(ctx, log, node); err != nil {
				return reconcile.Result{}, err
			}
		case node.Labels[machinev1alpha1.LabelKeyNodeUpdateResult] == machinev1alpha1.LabelValueNodeUpdateFailed:
			if err := r.handleUpdateFailed(ctx, log, node); err != nil {
				return reconcile.Result{}, err
			}
		case r.isUpdateTimedOut(node):
			if err := r.markUpdateTimedOut(ctx, log, node); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	if err := r.SeedClient.List(ctx, nodeList, client.MatchingLabels{
		v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: poolSecretName,
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to re-list nodes for pool %s: %w", poolSecretName, err)
	}

	inProgress := 0
	for i := range nodeList.Items {
		if NodeIsUnavailableForInPlaceUpdate(&nodeList.Items[i]) {
			inProgress++
		}
	}

	for i := range nodeList.Items {
		if inProgress >= maxUnavailable {
			break
		}
		node := &nodeList.Items[i]
		if node.Annotations[v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain] != "true" || NodeIsUnavailableForInPlaceUpdate(node) {
			continue
		}
		patch := client.MergeFrom(node.DeepCopy())
		node.Spec.Unschedulable = true
		metav1.SetMetaDataAnnotation(&node.ObjectMeta, v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime, r.Clock.Now().UTC().Format(time.RFC3339))
		if err := r.SeedClient.Patch(ctx, node, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to cordon node %s: %w", node.Name, err)
		}
		log.Info("Cordoned node, starting drain", "node", node.Name)
		inProgress++
	}

	needsRequeue := false
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if node.Annotations[v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime] == "" || !node.Spec.Unschedulable {
			continue
		}

		drainTimedOut := r.isDrainTimedOut(log, node)

		if err := r.evictPods(ctx, log, node, drainTimedOut); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to evict pods from node %s: %w", node.Name, err)
		}

		remaining, err := r.listEvictablePods(ctx, node.Name)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to list remaining pods on node %s: %w", node.Name, err)
		}

		if len(remaining) > 0 {
			if !drainTimedOut {
				log.Info("Pods still terminating on node, will retry", "node", node.Name, "count", len(remaining))
				needsRequeue = true
				continue
			}
			log.Info("Drain timed out, force-deleting remaining pods", "node", node.Name, "count", len(remaining))
			for _, pod := range remaining {
				if err := r.SeedClient.Delete(ctx, &pod, client.GracePeriodSeconds(0)); client.IgnoreNotFound(err) != nil {
					return reconcile.Result{}, fmt.Errorf("failed to force-delete pod %s/%s: %w", pod.Namespace, pod.Name, err)
				}
			}
		}

		condPatch := client.MergeFrom(node.DeepCopy())
		r.setNodeInPlaceUpdateCondition(node, machinev1alpha1.ReadyForUpdate, "Node drained and ready for in-place update")
		if err := r.SeedClient.Status().Patch(ctx, node, condPatch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to set NodeInPlaceUpdate condition on node %s: %w", node.Name, err)
		}

		annotPatch := client.MergeFrom(node.DeepCopy())
		delete(node.Annotations, v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain)
		delete(node.Annotations, v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime)
		if err := r.SeedClient.Patch(ctx, node, annotPatch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove drain annotations from node %s: %w", node.Name, err)
		}
		log.Info("Drain complete, node ready for in-place update", "node", node.Name)
	}

	if needsRequeue {
		return reconcile.Result{RequeueAfter: podEvictionRetryInterval}, nil
	}

	// If any node is waiting for GNA to finish the update (ReadyForUpdate condition set),
	// requeue just before the earliest timeout so we can detect a stuck GNA.
	var requeueAfter time.Duration
	for i := range nodeList.Items {
		for _, cond := range nodeList.Items[i].Status.Conditions {
			if cond.Type != machinev1alpha1.NodeInPlaceUpdate ||
				cond.Status != corev1.ConditionTrue ||
				cond.Reason != machinev1alpha1.ReadyForUpdate {
				continue
			}
			remaining := updateTimeout - r.Clock.Since(cond.LastTransitionTime.Time)
			if remaining > 0 && (requeueAfter == 0 || remaining < requeueAfter) {
				requeueAfter = remaining
			}
		}
	}
	if requeueAfter > 0 {
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) cleanupAfterSuccessfulUpdate(ctx context.Context, log logr.Logger, node *corev1.Node) error {
	log = log.WithValues("node", node.Name)

	// Delete gardener-resource-manager pods so they restart and pick up updated configuration
	// (new CA bundles, SA keys, etc.) from the in-place update. GNA skips GRM during pod
	// deletion to avoid a webhook deadlock, so the gardenlet restarts it here instead.
	if r.ControlPlaneNamespace != "" {
		podList := &corev1.PodList{}
		if err := r.SeedClient.List(ctx, podList,
			client.InNamespace(r.ControlPlaneNamespace),
			client.MatchingLabels{v1beta1constants.LabelApp: v1beta1constants.DeploymentNameGardenerResourceManager},
		); err != nil {
			return fmt.Errorf("failed listing gardener-resource-manager pods in namespace %s: %w", r.ControlPlaneNamespace, err)
		}
		for i := range podList.Items {
			if err := r.SeedClient.Delete(ctx, &podList.Items[i]); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed deleting gardener-resource-manager pod %s: %w", podList.Items[i].Name, err)
			}
		}
		if len(podList.Items) > 0 {
			log.Info("Deleted gardener-resource-manager pods to pick up updated configuration")
		}
	}

	condPatch := client.MergeFrom(node.DeepCopy())
	r.setNodeInPlaceUpdateCondition(node, machinev1alpha1.UpdateSuccessful, "In-place update completed successfully")
	if err := r.SeedClient.Status().Patch(ctx, node, condPatch); err != nil {
		return fmt.Errorf("failed to set NodeInPlaceUpdate condition to successful on node %s: %w", node.Name, err)
	}

	labelPatch := client.MergeFrom(node.DeepCopy())
	delete(node.Labels, machinev1alpha1.LabelKeyNodeUpdateResult)
	if err := r.SeedClient.Patch(ctx, node, labelPatch); err != nil {
		return fmt.Errorf("failed to remove update-result label from node %s: %w", node.Name, err)
	}
	log.Info("Cleaned up node after successful in-place update")
	return nil
}

// handleUpdateFailed handles the case where GNA has set the update-result=failed label.
func (r *Reconciler) handleUpdateFailed(ctx context.Context, log logr.Logger, node *corev1.Node) error {
	log = log.WithValues("node", node.Name)
	log.Info("GNA reported update failed, recording condition")

	reason := node.Annotations[machinev1alpha1.AnnotationKeyMachineUpdateFailedReason]
	if reason == "" {
		reason = "GNA reported in-place update failure"
	}

	patch := client.MergeFrom(node.DeepCopy())
	r.setNodeInPlaceUpdateCondition(node, machinev1alpha1.UpdateFailed, reason)
	if err := r.SeedClient.Status().Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("failed to update NodeInPlaceUpdate condition to failed on node %s: %w", node.Name, err)
	}

	log.Info("Recorded GNA-reported update failure in condition")
	return nil
}

func (r *Reconciler) isUpdateTimedOut(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == machinev1alpha1.NodeInPlaceUpdate && cond.Status == corev1.ConditionTrue && cond.Reason == machinev1alpha1.ReadyForUpdate {
			return r.Clock.Since(cond.LastTransitionTime.Time) > updateTimeout
		}
	}
	return false
}

func (r *Reconciler) markUpdateTimedOut(ctx context.Context, log logr.Logger, node *corev1.Node) error {
	log = log.WithValues("node", node.Name)
	log.Info("In-place update timed out, marking update as failed")

	labelPatch := client.MergeFrom(node.DeepCopy())
	metav1.SetMetaDataLabel(&node.ObjectMeta, machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateFailed)
	if err := r.SeedClient.Patch(ctx, node, labelPatch); err != nil {
		return fmt.Errorf("failed to label node with update failed label %s: %w", node.Name, err)
	}

	patch := client.MergeFrom(node.DeepCopy())
	r.setNodeInPlaceUpdateCondition(node, machinev1alpha1.UpdateFailed, "GNA failed to complete the in-place update within the expected time")
	if err := r.SeedClient.Status().Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("failed to update NodeInPlaceUpdate condition to failed on node %s: %w", node.Name, err)
	}

	log.Info("Marked in-place update as failed due to timeout")
	return nil
}

// evictPods attempts to evict all evictable pods from the node. If drainTimedOut is true,
// pods that are PDB-protected are force-deleted instead of retried.
func (r *Reconciler) evictPods(ctx context.Context, log logr.Logger, node *corev1.Node, drainTimedOut bool) error {
	log = log.WithValues("node", node.Name)

	pods, err := r.listEvictablePods(ctx, node.Name)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		eviction := &policyv1.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			},
		}
		if err := r.SeedClient.SubResource("eviction").Create(ctx, &pod, eviction); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			if apierrors.IsTooManyRequests(err) {
				if drainTimedOut {
					log.Info("Drain timed out, force-deleting PDB-protected pod", "pod", client.ObjectKeyFromObject(&pod))
					if err := r.SeedClient.Delete(ctx, &pod, client.GracePeriodSeconds(0)); client.IgnoreNotFound(err) != nil {
						return fmt.Errorf("failed to force-delete pod %s/%s: %w", pod.Namespace, pod.Name, err)
					}
				} else {
					log.V(1).Info("Pod eviction blocked by PDB, will retry", "pod", client.ObjectKeyFromObject(&pod))
				}
				continue
			}
			return fmt.Errorf("failed to evict pod %s/%s: %w", pod.Namespace, pod.Name, err)
		}
	}

	return nil
}

func (r *Reconciler) isDrainTimedOut(log logr.Logger, node *corev1.Node) bool {
	startTimeStr, ok := node.Annotations[v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime]
	if !ok {
		return false
	}
	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		// The annotation value is malformed. Treat the drain as timed out so the node is
		// not stuck waiting forever; the force-delete path will unblock it.
		log.Error(err, "Failed to parse drain-start-time annotation, treating drain as timed out", "node", node.Name, "value", startTimeStr)
		return true
	}
	return r.Clock.Since(startTime) > drainTimeout
}

func (r *Reconciler) listEvictablePods(ctx context.Context, nodeName string) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := r.SeedClient.List(ctx, podList, client.MatchingFields{indexer.PodNodeName: nodeName}); err != nil {
		return nil, fmt.Errorf("failed to list pods on node %s: %w", nodeName, err)
	}

	var evictable []corev1.Pod
	for _, pod := range podList.Items {
		if ShouldSkipPod(&pod) {
			continue
		}
		evictable = append(evictable, pod)
	}
	return evictable, nil
}

// ShouldSkipPod returns true if the given pod should not be evicted during a node drain.
func ShouldSkipPod(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return true
	}
	if _, isMirror := pod.Annotations[corev1.MirrorPodAnnotationKey]; isMirror {
		return true
	}
	if controllerRef := metav1.GetControllerOf(pod); controllerRef != nil && controllerRef.Kind == "DaemonSet" {
		return true
	}
	// Skip the gardenlet pod, it performs the drain and must not evict itself.
	if pod.Labels[v1beta1constants.LabelRole] == v1beta1constants.DeploymentNameGardenlet {
		return true
	}
	// Skip gardener-resource-manager: its webhook validates all GNA API calls during the update.
	// Evicting it would make GNA unable to proceed with the in-place update.
	if pod.Labels[v1beta1constants.LabelApp] == v1beta1constants.DeploymentNameGardenerResourceManager {
		return true
	}
	// Skip pods that tolerate the unschedulable taint — they reschedule back onto the cordoned
	// node immediately after eviction, causing an infinite drain loop.
	if podToleratesUnschedulable(pod) {
		return true
	}
	return false
}

func podToleratesUnschedulable(pod *corev1.Pod) bool {
	for _, t := range pod.Spec.Tolerations {
		if t.Effect == corev1.TaintEffectNoSchedule &&
			(t.Key == corev1.TaintNodeUnschedulable || t.Key == "") &&
			(t.Operator == corev1.TolerationOpExists) {
			return true
		}
	}
	return false
}

// NodeIsUnavailableForInPlaceUpdate returns true if the node is currently unavailable for
// in-place update purposes (cordoned and draining/conditioned, or marked failed).
func NodeIsUnavailableForInPlaceUpdate(node *corev1.Node) bool {
	if node.Spec.Unschedulable && (node.Annotations[v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime] != "" || nodeHasInPlaceUpdateCondition(node)) {
		return true
	}
	if node.Labels[machinev1alpha1.LabelKeyNodeUpdateResult] == machinev1alpha1.LabelValueNodeUpdateFailed {
		return true
	}
	return false
}

func nodeHasInPlaceUpdateCondition(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == machinev1alpha1.NodeInPlaceUpdate && cond.Reason != machinev1alpha1.UpdateSuccessful {
			return true
		}
	}
	return false
}

// MaxUnavailableForPool returns the maximum number of nodes that can undergo in-place
// updates simultaneously for the given pool. Control plane pools are always limited to 1.
func (r *Reconciler) MaxUnavailableForPool(poolName string, currentNodeCount int) int {
	for _, w := range r.Workers {
		if w.Name != poolName {
			continue
		}
		if w.ControlPlane != nil {
			return 1
		}
		if w.MaxUnavailable != nil {
			maxUnavailable, err := intstr.GetScaledValueFromIntOrPercent(w.MaxUnavailable, currentNodeCount, false)
			if err == nil && maxUnavailable > 0 {
				return maxUnavailable
			}
		}
	}
	return 1
}

func (r *Reconciler) setNodeInPlaceUpdateCondition(node *corev1.Node, reason, message string) {
	now := metav1.NewTime(r.Clock.Now())
	for i, cond := range node.Status.Conditions {
		if cond.Type == machinev1alpha1.NodeInPlaceUpdate {
			if cond.Status != corev1.ConditionTrue || cond.Reason != reason {
				node.Status.Conditions[i].LastTransitionTime = now
			}
			node.Status.Conditions[i].Status = corev1.ConditionTrue
			node.Status.Conditions[i].Reason = reason
			node.Status.Conditions[i].Message = message
			return
		}
	}
	node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
		Type:               machinev1alpha1.NodeInPlaceUpdate,
		Status:             corev1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	})
}
