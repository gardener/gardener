// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package bootstrappers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/cmd/gardenlet/app/bootstrappers"
	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
)

var _ = Describe("SeedConfigChecker", func() {
	Describe("#Start", func() {
		var (
			ctx     = context.TODO()
			client  client.Client
			checker *SeedConfigChecker

			podCIDR     = "10.0.0.0/8"
			serviceCIDR = "192.168.0.0/16"
			nodeCIDR    = "172.16.0.0/12"
			otherCIDR   = "1.1.0.0/22"

			shootInfoConfigMap *corev1.ConfigMap
			shootInfoWithNodes = map[string]string{
				"podNetwork":     podCIDR,
				"serviceNetwork": serviceCIDR,
				"nodeNetwork":    nodeCIDR,
			}
			shootInfoWithIncorrectNodes = map[string]string{
				"podNetwork":     podCIDR,
				"serviceNetwork": serviceCIDR,
				"nodeNetwork":    otherCIDR,
			}
			shootInfoWithoutNodes = map[string]string{
				"podNetwork":     podCIDR,
				"serviceNetwork": serviceCIDR,
			}

			node = corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-node",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{
						Type:    corev1.NodeInternalIP,
						Address: "172.16.10.10",
					}},
				},
			}

			pendingNode = corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-pending-node",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{},
				},
			}

			incorrectNode = corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-incorrect-node",
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{{
						Type:    corev1.NodeInternalIP,
						Address: "1.1.10.10",
					}},
				},
			}

			pod = corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-pod",
					Namespace: metav1.NamespaceSystem,
				},
				Spec: corev1.PodSpec{
					HostNetwork: false,
				},
				Status: corev1.PodStatus{
					PodIP: "10.10.10.10",
				},
			}

			pendingPod = corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-pending-pod",
					Namespace: metav1.NamespaceSystem,
				},
				Spec: corev1.PodSpec{
					HostNetwork: false,
				},
				Status: corev1.PodStatus{
					PodIP: "",
				},
			}

			incorrectPod = corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-incorrect-pod",
					Namespace: metav1.NamespaceSystem,
				},
				Spec: corev1.PodSpec{
					HostNetwork: false,
				},
				Status: corev1.PodStatus{
					PodIP: "1.1.10.10",
				},
			}

			incorrectHostNetworkPod = corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-incorrect-hostnetwork-pod",
					Namespace: metav1.NamespaceSystem,
				},
				Spec: corev1.PodSpec{
					HostNetwork: true,
				},
				Status: corev1.PodStatus{
					PodIP: "1.1.10.10",
				},
			}

			service = corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-service",
					Namespace: metav1.NamespaceSystem,
				},
				Spec: corev1.ServiceSpec{
					Type:      corev1.ServiceTypeClusterIP,
					ClusterIP: "192.168.10.10",
				},
			}

			pendingService = corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-pending-service",
					Namespace: metav1.NamespaceSystem,
				},
				Spec: corev1.ServiceSpec{
					Type:      corev1.ServiceTypeClusterIP,
					ClusterIP: "",
				},
			}

			incorrectService = corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-incorrect-service",
					Namespace: metav1.NamespaceSystem,
				},
				Spec: corev1.ServiceSpec{
					Type:      corev1.ServiceTypeClusterIP,
					ClusterIP: "1.1.10.10",
				},
			}

			headlessService = corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-headless-service",
					Namespace: metav1.NamespaceSystem,
				},
				Spec: corev1.ServiceSpec{
					Type:      corev1.ServiceTypeClusterIP,
					ClusterIP: corev1.ClusterIPNone,
				},
			}

			loadBalancerService = corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-loadbalancer-service",
					Namespace: metav1.NamespaceSystem,
				},
				Spec: corev1.ServiceSpec{
					Type:      corev1.ServiceTypeLoadBalancer,
					ClusterIP: "",
				},
			}
		)

		BeforeEach(func() {
			client = fakeclient.NewClientBuilder().Build()
			checker = &SeedConfigChecker{SeedClient: client}

			shootInfoConfigMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      v1beta1constants.ConfigMapNameShootInfo,
					Namespace: metav1.NamespaceSystem,
				},
			}
		})

		DescribeTable("validate seed network configuration",
			func(seedConfig *config.SeedConfig, shootInfoData map[string]string, secretRetrievalExpected bool, matcher gomegatypes.GomegaMatcher) {
				checker.SeedConfig = seedConfig

				shootInfoConfigMap.Data = shootInfoData
				Expect(client.Create(ctx, shootInfoConfigMap)).To(Succeed())

				Expect(checker.Start(ctx)).To(matcher)
			},

			Entry("no seed configuration", nil, shootInfoWithNodes, false, BeNil()),
			Entry("correct seed configuration with nodes", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithNodes, true, BeNil()),
			Entry("correct seed configuration without nodes", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithoutNodes, true, BeNil()),
			Entry("correct seed configuration with nodes but no nodes in shoot-info", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithoutNodes, true, BeNil()),
			Entry("correct seed configuration without nodes but nodes in shoot-info", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithNodes, true, BeNil()),
			Entry("correct seed configuration with incorrect nodes but no nodes in shoot-info", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &otherCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithoutNodes, true, BeNil()),
			Entry("correct seed configuration without nodes but incorrect nodes in shoot-info", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithIncorrectNodes, true, BeNil()),
			Entry("incorrect node cidr", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &otherCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithNodes, true, HaveOccurred()),
			Entry("incorrect pod cidr", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     otherCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithNodes, true, HaveOccurred()),
			Entry("incorrect service cidr", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: otherCIDR,
			}}}}, shootInfoWithNodes, true, HaveOccurred()),
		)

		DescribeTable("validate seed network configuration heuristically",
			func(seedConfig *config.SeedConfig, nodes []corev1.Node, pods []corev1.Pod, services []corev1.Service, matcher gomegatypes.GomegaMatcher) {
				checker.SeedConfig = seedConfig

				for _, n := range nodes {
					Expect(client.Create(ctx, &n)).To(Succeed())
				}

				for _, p := range pods {
					Expect(client.Create(ctx, &p)).To(Succeed())
				}

				for _, s := range services {
					Expect(client.Create(ctx, &s)).To(Succeed())
				}

				Expect(checker.Start(ctx)).To(matcher)
			},

			Entry("correct seed configuration with nodes", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, []corev1.Node{pendingNode, node}, []corev1.Pod{pendingPod, incorrectHostNetworkPod, pod}, []corev1.Service{pendingService, headlessService, loadBalancerService, service}, BeNil()),
			Entry("incorrect node", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, []corev1.Node{pendingNode, node, incorrectNode}, []corev1.Pod{pendingPod, incorrectHostNetworkPod, pod}, []corev1.Service{pendingService, headlessService, loadBalancerService, service}, HaveOccurred()),
			Entry("incorrect pod", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, []corev1.Node{pendingNode, node}, []corev1.Pod{pendingPod, incorrectHostNetworkPod, pod, incorrectPod}, []corev1.Service{pendingService, headlessService, loadBalancerService, service}, HaveOccurred()),
			Entry("incorrect service", &config.SeedConfig{SeedTemplate: core.SeedTemplate{Spec: core.SeedSpec{Networks: core.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, []corev1.Node{pendingNode, node}, []corev1.Pod{pendingPod, incorrectHostNetworkPod, pod}, []corev1.Service{pendingService, headlessService, loadBalancerService, service, incorrectService}, HaveOccurred()),
		)
	})
})
