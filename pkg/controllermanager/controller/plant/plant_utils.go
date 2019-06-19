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
	"strings"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
)

// Unknown is a constant to be used for unknown cloud info
const Unknown = "<unknown>"

// FetchCloudInfo deduces the cloud info from the plant cluster
func FetchCloudInfo(ctx context.Context, plantClient client.Client, discoveryClient discovery.DiscoveryInterface, logger logrus.FieldLogger) (*StatusCloudInfo, error) {
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
	nodes := &corev1.NodeList{}
	if err := cl.List(ctx, nodes, kutil.Limit(1)); err != nil {
		return nil, err
	}

	if len(nodes.Items) == 0 {
		return &StatusCloudInfo{
			CloudType: Unknown,
			Region:    Unknown,
		}, nil
	}

	var (
		firstNode = nodes.Items[0]
		region    = getRegionNameForNode(firstNode)
		provider  = getCloudProviderForNode(firstNode.Spec.ProviderID)
	)

	return &StatusCloudInfo{
		Region:    region,
		CloudType: provider,
	}, nil
}

func getCloudProviderForNode(providerID string) string {
	provider := strings.Split(providerID, "://")
	if len(provider) == 1 && len(providerID) == 0 {
		return Unknown
	}
	return provider[0]
}

func getRegionNameForNode(node corev1.Node) string {
	for key, value := range node.Labels {
		if key == corev1.LabelZoneRegion {
			return value
		}
		if key == corev1.LabelZoneFailureDomain {
			return value
		}
	}
	return Unknown
}

func isPlantSecret(plant *gardencorev1alpha1.Plant, secretKey client.ObjectKey) bool {
	return plant.Spec.SecretRef.Name == secretKey.Name && plant.Namespace == secretKey.Namespace
}
