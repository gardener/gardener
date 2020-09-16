// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plant

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewHealthChecker creates a new health checker.
func NewHealthChecker(plantClient client.Client, discoveryClient discovery.DiscoveryInterface) *HealthChecker {
	return &HealthChecker{
		plantClient:     plantClient,
		discoveryClient: discoveryClient,
	}
}

// HealthChecker contains the condition thresholds.
type HealthChecker struct {
	plantClient     client.Client
	discoveryClient discovery.DiscoveryInterface
}

// CheckPlantClusterNodes checks whether cluster nodes in the given listers are complete and healthy.
func (h *HealthChecker) CheckPlantClusterNodes(ctx context.Context, condition gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	nodeList := &corev1.NodeList{}
	err := h.plantClient.List(ctx, nodeList)
	if err != nil {
		return gardencorev1beta1helper.UpdatedConditionUnknownError(condition, err)
	}

	if exitCondition, err := h.checkNodes(condition, nodeList); err != nil {
		return exitCondition
	}

	updatedCondition := gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionTrue, string(gardencorev1beta1.PlantEveryNodeReady), "Every node registered to the cluster is ready.")
	return updatedCondition
}

// CheckAPIServerAvailability checks if the API server of a Plant cluster is reachable and measure the response time.
func (h *HealthChecker) CheckAPIServerAvailability(ctx context.Context, condition gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	return health.CheckAPIServerAvailability(ctx, condition, h.discoveryClient.RESTClient(), func(conditionType, message string) gardencorev1beta1.Condition {
		return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionFalse, conditionType, message)
	})
}

func (h *HealthChecker) checkNodes(condition gardencorev1beta1.Condition, nodeList *corev1.NodeList) (gardencorev1beta1.Condition, error) {
	for _, object := range nodeList.Items {
		if err := health.CheckNode(&object); err != nil {
			failedCondition := gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionFalse, "NodesUnhealthy", fmt.Sprintf("Node %s is unhealthy: %v", object.Name, err))
			return failedCondition, err
		}
	}
	return condition, nil
}
