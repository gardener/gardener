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
	"strings"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Following labels come from k8s.io/kubernetes/pkg/kubelet/apis

	// LabelZoneRegion zone Region label
	LabelZoneRegion = "failure-domain.beta.kubernetes.io/region"
)

func newConditionOrError(oldCondition, newCondition gardencorev1alpha1.Condition, err error) gardencorev1alpha1.Condition {
	if err != nil {
		return helper.UpdatedConditionUnknownError(oldCondition, err)
	}
	return newCondition
}

// FetchCloudInfo deduces the cloud info from the plant cluster
func FetchCloudInfo(ctx context.Context, plantClient client.Client, discoveryClient discovery.DiscoveryInterface, logger logrus.FieldLogger) (*StatusCloudInfo, error) {
	if plantClient == nil || discoveryClient == nil {
		return nil, fmt.Errorf("plant clients need to be initialized first")
	}

	cloudInfo, err := getClusterInfo(ctx, plantClient, logger)
	if err != nil {
		return nil, err
	}

	kubernetesVersionInfo, err := discoveryClient.ServerVersion()
	if err != nil {
		return nil, err
	}
	cloudInfo.K8sVersion = kubernetesVersionInfo.String()

	return cloudInfo, nil
}

// getClusterInfo gets the kubernetes cluster zones and Region by inspecting labels on nodes in the cluster.
func getClusterInfo(ctx context.Context, cl client.Client, logger logrus.FieldLogger) (*StatusCloudInfo, error) {
	var nodes = &corev1.NodeList{}
	err := cl.List(ctx, &client.ListOptions{}, nodes)
	if err != nil {
		logger.Errorf("Failed to list nodes while getting cluster Info: %v", err)
		return nil, err
	}

	if len(nodes.Items) == 0 {
		return nil, fmt.Errorf("there are no nodes available in this cluster to retrieve zones and regions from")
	}

	// we are only taking the first node because all nodes that
	firstNode := nodes.Items[0]
	region, err := getRegionNameForNode(firstNode)
	if err != nil {
		return nil, err
	}

	provider := getCloudProviderForNode(firstNode.Spec.ProviderID)
	return &StatusCloudInfo{
		Region:    region,
		CloudType: provider,
	}, nil
}

func getCloudProviderForNode(providerID string) string {
	provider := strings.Split(providerID, "://")
	if len(provider) == 1 && len(providerID) == 0 {
		return "<unknown>"
	}
	return provider[0]
}

func getRegionNameForNode(node corev1.Node) (string, error) {
	for key, value := range node.Labels {
		// TODO: replace LabelZoneRegion const with corev1.LabelZoneRegion which will be availbale in 1.14
		if key == LabelZoneRegion {
			return value, nil
		}
	}
	return "", errors.Errorf("Region name for node %s not found. No label with key %s", node.Name, LabelZoneRegion)
}

func resetClients(plantControl *defaultPlantControl, key string) {
	plantControl.plantClient[key] = nil
	plantControl.discoveryClient[key] = nil
}
