// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plant

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/client-go/discovery"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"

	"github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
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
func (h *HealthChecker) CheckPlantClusterNodes(ctx context.Context, condition gardencorev1alpha1.Condition) gardencorev1alpha1.Condition {
	nodeList := &corev1.NodeList{}
	err := h.plantClient.List(ctx, nodeList)
	if err != nil {
		return helper.UpdatedConditionUnknownError(condition, err)
	}

	if exitCondition, err := h.checkNodes(condition, nodeList); err != nil {
		return exitCondition
	}

	updatedCondition := helper.UpdatedCondition(condition, gardencorev1alpha1.ConditionTrue, string(gardencorev1alpha1.PlantEveryNodeReady), "Every node registered to the cluster is ready")
	return updatedCondition
}

// CheckAPIServerAvailability checks if the API server of a Plant cluster is reachable and measure the response time.
func (h *HealthChecker) CheckAPIServerAvailability(condition gardencorev1alpha1.Condition) gardencorev1alpha1.Condition {
	return health.CheckAPIServerAvailability(condition, h.discoveryClient.RESTClient(), func(conditionType, message string) gardencorev1alpha1.Condition {
		return helper.UpdatedCondition(condition, gardencorev1alpha1.ConditionFalse, conditionType, message)
	})
}

func (h *HealthChecker) checkNodes(condition gardencorev1alpha1.Condition, nodeList *corev1.NodeList) (gardencorev1alpha1.Condition, error) {
	for _, object := range nodeList.Items {
		if err := health.CheckNode(&object); err != nil {
			failedCondition := helper.UpdatedCondition(condition, gardencorev1alpha1.ConditionFalse, "NodesUnhealthy", fmt.Sprintf("Node %s is unhealthy: %v", object.Name, err))
			return failedCondition, err
		}
	}
	return condition, nil
}

func (h *HealthChecker) makePlantNodeLister(ctx context.Context, options *client.ListOptions) kutil.NodeLister {
	var (
		once  sync.Once
		items []*corev1.Node
		err   error

		onceBody = func() {
			nodeList := &corev1.NodeList{}
			err = h.plantClient.List(ctx, nodeList, client.UseListOptions(options))
			if err != nil {
				return
			}

			for _, item := range nodeList.Items {
				it := item
				items = append(items, &it)
			}
		}
	)

	return kutil.NewNodeLister(func() ([]*corev1.Node, error) {
		once.Do(onceBody)
		return items, err
	})
}
