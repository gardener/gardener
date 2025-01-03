// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
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
			func(seedConfig *gardenletconfigv1alpha1.SeedConfig, shootInfoData map[string]string, matcher gomegatypes.GomegaMatcher) {
				checker.SeedConfig = seedConfig

				shootInfoConfigMap.Data = shootInfoData
				Expect(client.Create(ctx, shootInfoConfigMap)).To(Succeed())

				Expect(checker.Start(ctx)).To(matcher)
			},

			Entry("no seed configuration", nil, shootInfoWithNodes, BeNil()),
			Entry("correct seed configuration with nodes", &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: gardencorev1beta1.SeedTemplate{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithNodes, BeNil()),
			Entry("correct seed configuration without nodes", &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: gardencorev1beta1.SeedTemplate{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithoutNodes, BeNil()),
			Entry("correct seed configuration with nodes but no nodes in shoot-info", &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: gardencorev1beta1.SeedTemplate{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithoutNodes, BeNil()),
			Entry("correct seed configuration without nodes but nodes in shoot-info", &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: gardencorev1beta1.SeedTemplate{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithNodes, BeNil()),
			Entry("correct seed configuration with incorrect nodes but no nodes in shoot-info", &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: gardencorev1beta1.SeedTemplate{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{
				Nodes:    &otherCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithoutNodes, BeNil()),
			Entry("correct seed configuration without nodes but incorrect nodes in shoot-info", &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: gardencorev1beta1.SeedTemplate{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithIncorrectNodes, BeNil()),
			Entry("incorrect node cidr", &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: gardencorev1beta1.SeedTemplate{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{
				Nodes:    &otherCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithNodes, HaveOccurred()),
			Entry("incorrect pod cidr", &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: gardencorev1beta1.SeedTemplate{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     otherCIDR,
				Services: serviceCIDR,
			}}}}, shootInfoWithNodes, HaveOccurred()),
			Entry("incorrect service cidr", &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: gardencorev1beta1.SeedTemplate{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: otherCIDR,
			}}}}, shootInfoWithNodes, HaveOccurred()),
		)

		DescribeTable("validate seed network configuration heuristically",
			func(seedConfig *gardenletconfigv1alpha1.SeedConfig, nodes []corev1.Node, pods []corev1.Pod, services []corev1.Service, matcher gomegatypes.GomegaMatcher) {
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

			Entry("correct seed configuration with nodes", &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: gardencorev1beta1.SeedTemplate{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, []corev1.Node{pendingNode, node}, []corev1.Pod{pendingPod, incorrectHostNetworkPod, pod}, []corev1.Service{pendingService, headlessService, loadBalancerService, service}, BeNil()),
			Entry("incorrect node", &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: gardencorev1beta1.SeedTemplate{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, []corev1.Node{pendingNode, node, incorrectNode}, []corev1.Pod{pendingPod, incorrectHostNetworkPod, pod}, []corev1.Service{pendingService, headlessService, loadBalancerService, service}, HaveOccurred()),
			Entry("incorrect pod", &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: gardencorev1beta1.SeedTemplate{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, []corev1.Node{pendingNode, node}, []corev1.Pod{pendingPod, incorrectHostNetworkPod, pod, incorrectPod}, []corev1.Service{pendingService, headlessService, loadBalancerService, service}, HaveOccurred()),
			Entry("incorrect service", &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: gardencorev1beta1.SeedTemplate{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{
				Nodes:    &nodeCIDR,
				Pods:     podCIDR,
				Services: serviceCIDR,
			}}}}, []corev1.Node{pendingNode, node}, []corev1.Pod{pendingPod, incorrectHostNetworkPod, pod}, []corev1.Service{pendingService, headlessService, loadBalancerService, service, incorrectService}, HaveOccurred()),
		)
	})
})
