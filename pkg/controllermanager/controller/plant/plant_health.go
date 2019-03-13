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
	"net/http"
	"sync"
	"time"

	"k8s.io/client-go/discovery"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"k8s.io/apimachinery/pkg/labels"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
)

// NewHealthCheker creates a new health checker.
func NewHealthCheker(plantClient client.Client, discoveryClient discovery.DiscoveryInterface) *HealthChecker {
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
func (h *HealthChecker) CheckPlantClusterNodes(condition *gardencorev1alpha1.Condition, nodeLister kutil.NodeLister) (*gardencorev1alpha1.Condition, error) {
	nodeList, err := nodeLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	if exitCondition := h.checkNodes(*condition, nodeList); exitCondition != nil {
		return exitCondition, nil
	}

	updatedCondition := helper.UpdatedCondition(*condition, corev1.ConditionTrue, string(gardencorev1alpha1.PlantEveryNodeReady), "Every node registered to the cluster is ready")

	return &updatedCondition, nil
}

// CheckAPIServerAvailability checks if the API server of a Plant cluster is reachable and measure the response time.
func (h *HealthChecker) CheckAPIServerAvailability(condition gardencorev1alpha1.Condition) gardencorev1alpha1.Condition {
	// Try to reach the Plant API server and measure the response time.
	now := time.Now()
	discoveryClient, ok := h.discoveryClient.(discovery.DiscoveryInterface)
	if !ok {
		message := fmt.Sprintf("discoveryClient does not implement discovery interface")
		return helper.UpdatedCondition(condition, corev1.ConditionFalse, "HealthzRequestFailed", message)
	}
	response := discoveryClient.RESTClient().Get().AbsPath("/healthz").Do()
	responseDurationText := fmt.Sprintf("[response_time:%dms]", time.Now().Sub(now).Nanoseconds()/time.Millisecond.Nanoseconds())
	if response.Error() != nil {
		message := fmt.Sprintf("Request to Plant API server /healthz endpoint failed. %s (%s)", responseDurationText, response.Error().Error())
		return helper.UpdatedCondition(condition, corev1.ConditionFalse, "HealthzRequestFailed", message)
	}

	// Determine the status code of the response.
	var statusCode int
	response.StatusCode(&statusCode)

	if statusCode != http.StatusOK {
		var body string
		bodyRaw, err := response.Raw()
		if err != nil {
			body = fmt.Sprintf("Could not parse response body: %s", err.Error())
		} else {
			body = string(bodyRaw)
		}
		message := fmt.Sprintf("Plant API server /healthz endpoint endpoint check returned a non ok status code %d. %s (%s)", statusCode, responseDurationText, body)
		return helper.UpdatedCondition(condition, corev1.ConditionFalse, "HealthzRequestError", message)
	}

	message := fmt.Sprintf("Plant API server /healthz endpoint responded with success status code. %s", responseDurationText)
	return helper.UpdatedCondition(condition, corev1.ConditionTrue, "HealthzRequestSucceeded", message)
}

func (h *HealthChecker) checkNodes(condition gardencorev1alpha1.Condition, objects []*corev1.Node) *gardencorev1alpha1.Condition {
	for _, object := range objects {
		if err := health.CheckNode(object); err != nil {
			failedCondition := helper.UpdatedCondition(condition, corev1.ConditionFalse, "NodesUnhealthy", fmt.Sprintf("Node %s is unhealthy: %v", object.Name, err))
			return &failedCondition
		}
	}
	return nil
}

func (h *HealthChecker) makePlantNodeLister(ctx context.Context, options *client.ListOptions) kutil.NodeLister {
	var (
		once  sync.Once
		items []*corev1.Node
		err   error

		onceBody = func() {
			nodeList := &corev1.NodeList{}
			err = h.plantClient.List(ctx, options, nodeList)
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
