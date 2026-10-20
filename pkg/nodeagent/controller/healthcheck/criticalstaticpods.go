// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/pkg/api/indexer"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// CriticalStaticPodsChecker configures the critical static pods healthcheck.
type CriticalStaticPodsChecker struct {
	client client.Client
	clock  clock.Clock
}

// NewCriticalStaticPodsChecker create an instance of a critical static pods health check.
func NewCriticalStaticPodsChecker(client client.Client, clock clock.Clock) *CriticalStaticPodsChecker {
	return &CriticalStaticPodsChecker{
		client: client,
		clock:  clock,
	}
}

// Name returns the name of this health check.
func (*CriticalStaticPodsChecker) Name() string {
	return "critical-static-pods"
}

// Check performs the actual health check for the critical static pods.
func (c *CriticalStaticPodsChecker) Check(ctx context.Context, node *corev1.Node) error {
	log := logf.FromContext(ctx).WithName(c.Name())
	podList := corev1.PodList{}
	if err := c.client.List(ctx, &podList, client.InNamespace(metav1.NamespaceSystem), client.MatchingFields{indexer.PodNodeName: node.Name}, client.MatchingLabels{v1beta1constants.LabelIsCriticalStaticPod: "true"}); err != nil {
		return err
	}

	log.V(1).Info("Listed critical static pods", "count", len(podList.Items))

	var err error
	for _, pod := range podList.Items {
		if err = health.CheckPod(&pod); err != nil {
			log.Error(err, "Critical static pod is unhealthy", "pod", pod.Name)
			break
		}
		if !health.IsPodReady(&pod) {
			err = fmt.Errorf("critical static pod %s is not ready", pod.Name)
			log.Error(err, "Critical static pod is not ready", "pod", pod.Name)
			break
		}
	}

	return c.updateNodeCondition(ctx, node, err)
}

func (c *CriticalStaticPodsChecker) updateNodeCondition(ctx context.Context, node *corev1.Node, err error) error {
	log := logf.FromContext(ctx).WithName(c.Name())
	var (
		// The JSON merge patch replaces the entire conditions list with the version of the (cached) node object. Use
		// optimistic locking to avoid dropping conditions that were concurrently added or updated by other components (e.g.,
		// the NetworkUnavailable condition managed by the CNI plugin). Conflicts are returned and retried.
		patch        = client.MergeFromWithOptions(node.DeepCopy(), client.MergeFromWithOptimisticLock{})
		now          = metav1.NewTime(c.clock.Now())
		newCondition corev1.NodeCondition
	)

	if err != nil {
		newCondition = corev1.NodeCondition{
			Type:    nodeagentconfigv1alpha1.ConditionTypeCriticalControlPlanePodsUnhealthy,
			Status:  corev1.ConditionTrue,
			Reason:  "SomeCriticalControlPlanePodsUnhealthy",
			Message: err.Error(),
		}
	} else {
		newCondition = corev1.NodeCondition{
			Type:    nodeagentconfigv1alpha1.ConditionTypeCriticalControlPlanePodsUnhealthy,
			Status:  corev1.ConditionFalse,
			Reason:  "AllCriticalControlPlanePodsHealthy",
			Message: "All critical control plane pods healthy.",
		}

	}

	existingIdx := slices.IndexFunc(node.Status.Conditions, func(c corev1.NodeCondition) bool {
		return c.Type == nodeagentconfigv1alpha1.ConditionTypeCriticalControlPlanePodsUnhealthy
	})

	if existingIdx >= 0 && node.Status.Conditions[existingIdx].Status == newCondition.Status {
		newCondition.LastTransitionTime = node.Status.Conditions[existingIdx].LastTransitionTime
	} else {
		log.Info("Critical static pods condition changed", "status", newCondition.Status, "reason", newCondition.Reason)
		newCondition.LastTransitionTime = now
	}
	newCondition.LastHeartbeatTime = now

	if existingIdx >= 0 {
		node.Status.Conditions[existingIdx] = newCondition
	} else {
		node.Status.Conditions = append(node.Status.Conditions, newCondition)
	}

	if err := c.client.Status().Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("failed patching node status with systemd unit check condition: %w", err)
	}

	return nil
}
