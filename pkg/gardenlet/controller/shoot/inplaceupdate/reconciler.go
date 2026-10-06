// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package inplaceupdate

import (
	"context"
	"fmt"
	"slices"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/extensions"
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

	return reconcile.Result{}, nil
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
