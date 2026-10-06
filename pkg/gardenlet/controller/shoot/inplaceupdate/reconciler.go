// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package inplaceupdate

import (
	"context"
	"fmt"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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
		}
	}

	return reconcile.Result{}, nil
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
