// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
