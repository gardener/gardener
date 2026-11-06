// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
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

// CriticalStaticPodsChecker configures the critical static pods health check.
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
	if err := c.client.List(ctx, &podList,
		client.InNamespace(metav1.NamespaceSystem),
		client.MatchingFields{indexer.PodNodeName: node.Name},
		client.MatchingLabels{v1beta1constants.LabelIsCriticalStaticPod: "true"},
	); err != nil {
		return fmt.Errorf("failed listing pods: %w", err)
	}

	log.V(1).Info("Listed critical static pods", "count", len(podList.Items))

	var err error
	for _, pod := range podList.Items {
		if err = health.CheckPod(&pod); err != nil {
			log.Error(err, "Critical static pod is unhealthy", "pod", client.ObjectKeyFromObject(&pod))
			break
		}
		if !health.IsPodReady(&pod) {
			err = fmt.Errorf("critical static pod %s is not ready", client.ObjectKeyFromObject(&pod))
			log.Error(err, "Critical static pod is not ready", "pod", client.ObjectKeyFromObject(&pod))
			break
		}
	}

	return c.updateNodeCondition(ctx, log, node, err)
}

func (c *CriticalStaticPodsChecker) updateNodeCondition(ctx context.Context, log logr.Logger, node *corev1.Node, unhealthyPodError error) error {
	var (
		now       = metav1.NewTime(c.clock.Now())
		condition = corev1.NodeCondition{
			Type:              nodeagentconfigv1alpha1.ConditionTypeCriticalControlPlanePodsUnhealthy,
			Status:            corev1.ConditionFalse,
			Reason:            "AllCriticalControlPlanePodsHealthy",
			Message:           "All critical control plane pods healthy.",
			LastHeartbeatTime: now,
		}
	)

	if unhealthyPodError != nil {
		condition.Status = corev1.ConditionTrue
		condition.Reason = "SomeCriticalControlPlanePodsUnhealthy"
		condition.Message = unhealthyPodError.Error()
	}

	idx := slices.IndexFunc(node.Status.Conditions, func(condition corev1.NodeCondition) bool {
		return condition.Type == nodeagentconfigv1alpha1.ConditionTypeCriticalControlPlanePodsUnhealthy
	})

	if idx != -1 && node.Status.Conditions[idx].Status == condition.Status {
		condition.LastTransitionTime = node.Status.Conditions[idx].LastTransitionTime
	} else {
		log.Info("Critical static pods condition changed", "status", condition.Status, "reason", condition.Reason)
		condition.LastTransitionTime = now
	}

	patch := client.StrategicMergeFrom(node.DeepCopy())
	if idx != -1 {
		node.Status.Conditions[idx] = condition
	} else {
		node.Status.Conditions = append(node.Status.Conditions, condition)
	}
	return c.client.Status().Patch(ctx, node, patch)
}
