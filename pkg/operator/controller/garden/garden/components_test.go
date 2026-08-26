// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

var _ = Describe("Components", func() {
	Describe("#virtualGardenAPIServerHost", func() {
		It("should return api.<domain> from the primary DNS domain", func() {
			garden := &operatorv1alpha1.Garden{
				Spec: operatorv1alpha1.GardenSpec{
					VirtualCluster: operatorv1alpha1.VirtualCluster{
						DNS: operatorv1alpha1.DNS{
							Domains: []operatorv1alpha1.DNSDomain{{Name: "garden.example.com"}},
						},
					},
				},
			}

			Expect(virtualGardenAPIServerHost(garden)).To(Equal("api.garden.example.com"))
		})

		It("should prefer the first SNI domain pattern when configured", func() {
			garden := &operatorv1alpha1.Garden{
				Spec: operatorv1alpha1.GardenSpec{
					VirtualCluster: operatorv1alpha1.VirtualCluster{
						DNS: operatorv1alpha1.DNS{
							Domains: []operatorv1alpha1.DNSDomain{{Name: "garden.example.com"}},
						},
						Kubernetes: operatorv1alpha1.Kubernetes{
							KubeAPIServer: &operatorv1alpha1.KubeAPIServerConfig{
								SNI: &operatorv1alpha1.SNI{
									DomainPatterns: []string{"api.custom.example.com", "api.other.example.com"},
								},
							},
						},
					},
				},
			}

			Expect(virtualGardenAPIServerHost(garden)).To(Equal("api.custom.example.com"))
		})
	})

	Describe("#getLoadBalancerServiceProxyProtocol", func() {
		DescribeTable("should return the proxy protocol setting",
			func(allowed bool) {
				garden := &operatorv1alpha1.Garden{
					Spec: operatorv1alpha1.GardenSpec{
						RuntimeCluster: operatorv1alpha1.RuntimeCluster{
							Settings: &operatorv1alpha1.Settings{
								LoadBalancerServices: &operatorv1alpha1.SettingLoadBalancerServices{
									ProxyProtocol: &operatorv1alpha1.LoadBalancerServicesProxyProtocol{
										Allowed: allowed,
									},
								},
							},
						},
					},
				}

				result := getLoadBalancerServiceProxyProtocol(garden)
				Expect(result).NotTo(BeNil())
				Expect(*result).To(Equal(allowed))
			},

			Entry("proxy protocol is allowed", true),
			Entry("proxy protocol is not allowed", false),
		)

		It("should return nil if ProxyProtocol is not set", func() {
			garden := &operatorv1alpha1.Garden{
				Spec: operatorv1alpha1.GardenSpec{
					RuntimeCluster: operatorv1alpha1.RuntimeCluster{
						Settings: &operatorv1alpha1.Settings{
							LoadBalancerServices: &operatorv1alpha1.SettingLoadBalancerServices{},
						},
					},
				},
			}

			Expect(getLoadBalancerServiceProxyProtocol(garden)).To(BeNil())
		})

		It("should return nil if LoadBalancerServices is not set", func() {
			garden := &operatorv1alpha1.Garden{
				Spec: operatorv1alpha1.GardenSpec{
					RuntimeCluster: operatorv1alpha1.RuntimeCluster{
						Settings: &operatorv1alpha1.Settings{},
					},
				},
			}

			Expect(getLoadBalancerServiceProxyProtocol(garden)).To(BeNil())
		})

		It("should return nil if Settings is not set", func() {
			garden := &operatorv1alpha1.Garden{}

			Expect(getLoadBalancerServiceProxyProtocol(garden)).To(BeNil())
		})
	})

	Describe("#getLoadBalancerServiceExternalTrafficPolicy", func() {
		DescribeTable("should return the external traffic policy setting",
			func(policy corev1.ServiceExternalTrafficPolicy) {
				garden := &operatorv1alpha1.Garden{
					Spec: operatorv1alpha1.GardenSpec{
						RuntimeCluster: operatorv1alpha1.RuntimeCluster{
							Settings: &operatorv1alpha1.Settings{
								LoadBalancerServices: &operatorv1alpha1.SettingLoadBalancerServices{
									ExternalTrafficPolicy: &policy,
								},
							},
						},
					},
				}

				result := getLoadBalancerServiceExternalTrafficPolicy(garden)
				Expect(result).NotTo(BeNil())
				Expect(*result).To(Equal(policy))
			},

			Entry("external traffic policy is Cluster", corev1.ServiceExternalTrafficPolicyCluster),
			Entry("external traffic policy is Local", corev1.ServiceExternalTrafficPolicyLocal),
		)

		It("should return nil if ExternalTrafficPolicy is not set", func() {
			garden := &operatorv1alpha1.Garden{
				Spec: operatorv1alpha1.GardenSpec{
					RuntimeCluster: operatorv1alpha1.RuntimeCluster{
						Settings: &operatorv1alpha1.Settings{
							LoadBalancerServices: &operatorv1alpha1.SettingLoadBalancerServices{},
						},
					},
				},
			}

			Expect(getLoadBalancerServiceExternalTrafficPolicy(garden)).To(BeNil())
		})

		It("should return nil if LoadBalancerServices is not set", func() {
			garden := &operatorv1alpha1.Garden{
				Spec: operatorv1alpha1.GardenSpec{
					RuntimeCluster: operatorv1alpha1.RuntimeCluster{
						Settings: &operatorv1alpha1.Settings{},
					},
				},
			}

			Expect(getLoadBalancerServiceExternalTrafficPolicy(garden)).To(BeNil())
		})

		It("should return nil if Settings is not set", func() {
			garden := &operatorv1alpha1.Garden{}

			Expect(getLoadBalancerServiceExternalTrafficPolicy(garden)).To(BeNil())
		})
	})
})
