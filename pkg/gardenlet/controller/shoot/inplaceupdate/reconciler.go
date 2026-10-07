// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package inplaceupdate

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/drain"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// Reconciler orchestrates in-place updates for all nodes in a worker pool.
type Reconciler struct {
	ShootClient         client.Client
	ShootClientSet      kubernetesclientset.Interface
	GardenClient        client.Client
	ShootNamespacedName client.ObjectKey
	Clock               clock.Clock
	Config              gardenletconfigv1alpha1.ShootInPlaceUpdateControllerConfiguration
}

// Reconcile processes all in-place-update state for a single worker pool.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)
	poolSecretName := req.Name

	nodeList := &corev1.NodeList{}
	if err := r.ShootClient.List(ctx, nodeList, client.MatchingLabels{v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: poolSecretName}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list nodes for pool %s: %w", poolSecretName, err)
	}
	if len(nodeList.Items) == 0 {
		return reconcile.Result{}, nil
	}

	for _, node := range nodeList.Items {
		switch {
		case node.Labels[machinev1alpha1.LabelKeyNodeUpdateResult] == machinev1alpha1.LabelValueNodeUpdateSuccessful:
			if err := r.cleanupAfterSuccessfulUpdate(ctx, log, &node); err != nil {
				return reconcile.Result{}, err
			}
		case node.Labels[machinev1alpha1.LabelKeyNodeUpdateResult] == machinev1alpha1.LabelValueNodeUpdateFailed:
			if err := r.handleUpdateFailed(ctx, log, &node); err != nil {
				return reconcile.Result{}, err
			}
		case r.isUpdateTimedOut(&node):
			if err := r.markUpdateTimedOut(ctx, log, &node); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	// Re-list the nodes to fetch the updates made by the previous loop, as it mutates node state (removes update-result labels,
	// updates conditions, uncordons completed nodes), and the logic below counts in-progress updates and decides which nodes to cordon
	// next based on that state.
	if err := r.ShootClient.List(ctx, nodeList, client.MatchingLabels{
		v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: poolSecretName,
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to re-list nodes for pool %s: %w", poolSecretName, err)
	}

	if err := r.cordonNodesForUpdate(ctx, log, nodeList); err != nil {
		return reconcile.Result{}, err
	}

	needsRequeue, err := r.drainPendingNodes(ctx, log, nodeList)
	if err != nil {
		return reconcile.Result{}, err
	}
	if needsRequeue {
		return reconcile.Result{RequeueAfter: r.Config.PodEvictionRetryInterval.Duration}, nil
	}

	return r.requeueForUpdateTimeout(nodeList), nil
}

// requeueForUpdateTimeout returns a Result that requeues just before the earliest ReadyForUpdate
// timeout across all nodes, so the next reconcile can detect a stuck GNA in time.
func (r *Reconciler) requeueForUpdateTimeout(nodeList *corev1.NodeList) reconcile.Result {
	var requeueAfter time.Duration
	for _, node := range nodeList.Items {
		for _, cond := range node.Status.Conditions {
			if cond.Type != machinev1alpha1.NodeInPlaceUpdate || cond.Status != corev1.ConditionTrue || cond.Reason != machinev1alpha1.ReadyForUpdate {
				continue
			}

			// Requeue instantly for nodes whose in-place update has already timed out.
			remaining := max(r.Config.UpdateTimeout.Duration-r.Clock.Since(cond.LastTransitionTime.Time), time.Second)
			if requeueAfter == 0 || remaining < requeueAfter {
				requeueAfter = remaining
			}
			break
		}
	}
	return reconcile.Result{RequeueAfter: requeueAfter}
}

func (r *Reconciler) drainPendingNodes(ctx context.Context, log logr.Logger, nodeList *corev1.NodeList) (bool, error) {
	var (
		drainFns     []flow.TaskFn
		requeueMu    sync.Mutex
		needsRequeue bool
	)

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if node.Annotations[v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime] == "" || !node.Spec.Unschedulable {
			continue
		}

		drainFns = append(drainFns, func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			if err := r.drainAndMarkReady(ctx, log, node); err != nil {
				if ctx.Err() != nil {
					return fmt.Errorf("context cancelled while draining node %s: %w", node.Name, ctx.Err())
				}
				log.Info("Drain not complete yet, will retry", "node", node.Name, "reason", err.Error())
				requeueMu.Lock()
				needsRequeue = true
				requeueMu.Unlock()
			}
			return nil
		})
	}

	if err := flow.Parallel(drainFns...)(ctx); err != nil {
		return false, err
	}

	return needsRequeue, nil
}

// NodeInPlaceUpdateOngoingOrFailed returns true if the node is currently unavailable for
// in-place update purposes (cordoned and draining/conditioned, or marked failed).
func NodeInPlaceUpdateOngoingOrFailed(node *corev1.Node) bool {
	return (node.Spec.Unschedulable && (node.Annotations[v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime] != "" || nodeHasInPlaceUpdateCondition(node))) ||
		node.Labels[machinev1alpha1.LabelKeyNodeUpdateResult] == machinev1alpha1.LabelValueNodeUpdateFailed
}

// MaxUnavailableForPool returns the maximum number of nodes that can undergo in-place
// updates simultaneously for the given pool. Control plane pools are always limited to 1.
func MaxUnavailableForPool(workers []gardencorev1beta1.Worker, poolName string, currentNodeCount int) int {
	for _, w := range workers {
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

// drainAndMarkReady drains the node and, on success, marks it ready for the in-place update (sets the ReadyForUpdate
// condition and removes the drain annotations).
func (r *Reconciler) drainAndMarkReady(ctx context.Context, log logr.Logger, node *corev1.Node) error {
	alreadyDrained := slices.ContainsFunc(node.Status.Conditions, func(cond corev1.NodeCondition) bool {
		return cond.Type == machinev1alpha1.NodeInPlaceUpdate && cond.Reason == machinev1alpha1.ReadyForUpdate
	})

	if !alreadyDrained {
		if err := r.drainNode(ctx, node); err != nil {
			return err
		}

		patch := client.MergeFrom(node.DeepCopy())
		r.setNodeInPlaceUpdateCondition(node, machinev1alpha1.ReadyForUpdate, "Node drained and ready for in-place update")
		if err := r.ShootClient.Status().Patch(ctx, node, patch); err != nil {
			return fmt.Errorf("failed to set NodeInPlaceUpdate condition on node %s: %w", node.Name, err)
		}
	}

	patch := client.MergeFrom(node.DeepCopy())
	delete(node.Annotations, v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain)
	delete(node.Annotations, v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime)
	if err := r.ShootClient.Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("failed to remove drain annotations from node %s: %w", node.Name, err)
	}

	log.Info("Drain complete, node ready for in-place update", "node", node.Name)
	return nil
}

// drainNode drains the node using the machine-controller-manager drain library's RunDrain. It gracefully evicts
// pods honoring PodDisruptionBudgets, retrying for up to DrainTimeout, and then force-deletes any pods still
// remaining.
//
// Volume attach/detach/reattach handling is skipped via SkipVolumeDetach: self-hosted shoots with
// unmanaged infrastructure run their own CSI drivers, so gardener cannot generically track volumes.
func (r *Reconciler) drainNode(ctx context.Context, node *corev1.Node) error {
	forceDeletePods := r.drainTimedOut(node)
	maxEvictRetries := int32(r.Config.DrainTimeout.Duration / r.Config.PodEvictionRetryInterval.Duration) // #nosec: G115

	drainOptions := drain.NewDrainOptions(
		r.ShootClientSet,
		nil,
		r.Config.DrainTimeout.Duration,
		maxEvictRetries,
		r.Config.DrainTimeout.Duration,
		r.Config.DrainTimeout.Duration,
		node.Name,
		-1,
		forceDeletePods,
		true,
		true,
		true,
		io.Discard,
		io.Discard,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	drainOptions.SkipVolumeDetach = true
	drainOptions.AdditionalPodFilters = []drain.AdditionalPodFilter{ShouldEvictPod}
	drainOptions.SetPodProvider(&podProvider{client: r.ShootClient})

	return drainOptions.RunDrain(ctx)
}

func (r *Reconciler) drainTimedOut(node *corev1.Node) bool {
	startStr := node.Annotations[v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime]
	if startStr == "" {
		return false
	}
	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return true
	}
	return r.Clock.Since(startTime) > r.Config.DrainTimeout.Duration
}

type podProvider struct {
	client client.Client
}

// PodsForNode returns all pods scheduled on the given node.
func (p *podProvider) PodsForNode(ctx context.Context, nodeName string) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := p.client.List(ctx, podList, client.MatchingFields{indexer.PodNodeName: nodeName}); err != nil {
		return nil, fmt.Errorf("failed to list pods on node %s: %w", nodeName, err)
	}
	return podList.Items, nil
}

// ShouldEvictPod returns true if the pod should be evicted during a node drain.
func ShouldEvictPod(pod corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return false
	}
	if pod.Labels[v1beta1constants.LabelRole] == v1beta1constants.DeploymentNameGardenlet {
		return false
	}
	if pod.Labels[v1beta1constants.LabelApp] == v1beta1constants.DeploymentNameGardenerResourceManager {
		return false
	}
	return !slices.ContainsFunc(pod.Spec.Tolerations, func(t corev1.Toleration) bool {
		return t.Effect == corev1.TaintEffectNoSchedule &&
			(t.Key == corev1.TaintNodeUnschedulable || t.Key == "") &&
			t.Operator == corev1.TolerationOpExists
	})
}

func (r *Reconciler) cordonNodesForUpdate(ctx context.Context, log logr.Logger, nodeList *corev1.NodeList) error {
	var (
		inProgress     int
		workers        = []gardencorev1beta1.Worker{}
		poolName       = nodeList.Items[0].Labels[v1beta1constants.LabelWorkerPool]
		maxUnavailable = MaxUnavailableForPool(workers, poolName, len(nodeList.Items))
	)

	workers, err := r.workersFromCluster(ctx)
	if err != nil {
		return err
	}

	for _, node := range nodeList.Items {
		if NodeInPlaceUpdateOngoingOrFailed(&node) {
			inProgress++
		}
	}

	for i := range nodeList.Items {
		if inProgress >= maxUnavailable {
			break
		}

		node := &nodeList.Items[i]
		if node.Annotations[v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain] != "true" || node.Annotations[v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime] != "" || NodeInPlaceUpdateOngoingOrFailed(node) {
			continue
		}

		patch := client.MergeFrom(node.DeepCopy())
		node.Spec.Unschedulable = true
		metav1.SetMetaDataAnnotation(&node.ObjectMeta, v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime, r.Clock.Now().UTC().Format(time.RFC3339))
		if err := r.ShootClient.Patch(ctx, node, patch); err != nil {
			return fmt.Errorf("failed to cordon node %s: %w", node.Name, err)
		}
		log.Info("Cordoned node for in-place update", "node", node.Name)
		inProgress++
	}

	return nil
}

func (r *Reconciler) workersFromCluster(ctx context.Context) ([]gardencorev1beta1.Worker, error) {
	cluster, err := extensions.GetCluster(ctx, r.ShootClient, metav1.NamespaceSystem)
	if err != nil {
		return nil, fmt.Errorf("failed getting Cluster resource for namespace %s: %w", metav1.NamespaceSystem, err)
	}
	if cluster.Shoot == nil {
		return nil, nil
	}
	return cluster.Shoot.Spec.Provider.Workers, nil
}

func (r *Reconciler) cleanupAfterSuccessfulUpdate(ctx context.Context, log logr.Logger, node *corev1.Node) error {
	log = log.WithValues("node", node.Name)

	patch := client.MergeFrom(node.DeepCopy())
	r.setNodeInPlaceUpdateCondition(node, machinev1alpha1.UpdateSuccessful, "In-place update completed successfully")
	if err := r.ShootClient.Status().Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("failed to set NodeInPlaceUpdate condition to successful on node %s: %w", node.Name, err)
	}

	patch = client.MergeFrom(node.DeepCopy())
	delete(node.Labels, machinev1alpha1.LabelKeyNodeUpdateResult)
	node.Spec.Unschedulable = false
	if err := r.ShootClient.Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("failed to remove update-result label and uncordon node %s: %w", node.Name, err)
	}
	log.Info("Cleaned up node after successful in-place update")
	return nil
}

// handleUpdateFailed handles the case where GNA has set the update-result=failed label.
func (r *Reconciler) handleUpdateFailed(ctx context.Context, log logr.Logger, node *corev1.Node) error {
	log = log.WithValues("node", node.Name)

	reason := node.Annotations[machinev1alpha1.AnnotationKeyMachineUpdateFailedReason]
	if reason == "" {
		reason = "GNA reported in-place update failure"
	}

	patch := client.MergeFrom(node.DeepCopy())
	r.setNodeInPlaceUpdateCondition(node, machinev1alpha1.UpdateFailed, reason)
	if err := r.ShootClient.Status().Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("failed to update NodeInPlaceUpdate condition to failed on node %s: %w", node.Name, err)
	}

	log.Info("Recorded GNA-reported update failure in condition", "reason", reason)
	return nil
}

func (r *Reconciler) isUpdateTimedOut(node *corev1.Node) bool {
	return slices.ContainsFunc(node.Status.Conditions, func(cond corev1.NodeCondition) bool {
		return cond.Type == machinev1alpha1.NodeInPlaceUpdate && cond.Status == corev1.ConditionTrue && cond.Reason == machinev1alpha1.ReadyForUpdate && r.Clock.Since(cond.LastTransitionTime.Time) > r.Config.UpdateTimeout.Duration
	})
}

func (r *Reconciler) markUpdateTimedOut(ctx context.Context, log logr.Logger, node *corev1.Node) error {
	log = log.WithValues("node", node.Name)

	patch := client.MergeFrom(node.DeepCopy())
	metav1.SetMetaDataLabel(&node.ObjectMeta, machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateFailed)
	if err := r.ShootClient.Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("failed to label node with update failed label %s: %w", node.Name, err)
	}

	patch = client.MergeFrom(node.DeepCopy())
	r.setNodeInPlaceUpdateCondition(node, machinev1alpha1.UpdateFailed, "GNA failed to complete the in-place update within the expected time")
	if err := r.ShootClient.Status().Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("failed to update NodeInPlaceUpdate condition to failed on node %s: %w", node.Name, err)
	}

	log.Info("Marked in-place update as failed due to timeout")
	return nil
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
