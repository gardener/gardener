// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("seed", func() {
	Describe("#GetLoadBalancerServiceAnnotations", func() {
		It("should return the annotations", func() {
			var (
				annotationKey1   = "my-annotation"
				annotationValue1 = "my-value"
				annotationKey2   = "second-annotation"
				annotationValue2 = "second-value"
				seed             = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
							Annotations: map[string]string{
								annotationKey1: annotationValue1,
								annotationKey2: annotationValue2,
							},
						},
					},
				},
			})

			Expect(seed.GetLoadBalancerServiceAnnotations()).ToNot(ShareSameReferenceAs(seed.GetInfo().Annotations))
			Expect(seed.GetLoadBalancerServiceAnnotations()).To(Equal(map[string]string{annotationKey1: annotationValue1, annotationKey2: annotationValue2}))
		})

		It("should return no annotations if no annotations are available", func() {
			var (
				seed = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
							Annotations: map[string]string{},
						},
					},
				},
			})

			Expect(seed.GetLoadBalancerServiceAnnotations()).ToNot(ShareSameReferenceAs(seed.GetInfo().Annotations))
			Expect(seed.GetLoadBalancerServiceAnnotations()).To(Equal(map[string]string{}))
		})

		It("should return no annotations if no settings are available", func() {
			var (
				seed = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{}})

			Expect(seed.GetLoadBalancerServiceAnnotations()).To(BeNil())
		})
	})

	Describe("#GetLoadBalancerServiceExternalTrafficPolicy", func() {
		It("should return the traffic policy", func() {
			var (
				policy = corev1.ServiceExternalTrafficPolicyLocal
				seed   = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
							ExternalTrafficPolicy: &policy,
						},
					},
				},
			})

			Expect(seed.GetLoadBalancerServiceExternalTrafficPolicy()).To(Equal(&policy))
		})

		It("should return no traffic policy if no is available", func() {
			var (
				seed = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{},
					},
				},
			})

			Expect(seed.GetLoadBalancerServiceExternalTrafficPolicy()).To(BeNil())
		})

		It("should return no traffic policy if no settings are available", func() {
			var (
				seed = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{}})

			Expect(seed.GetLoadBalancerServiceExternalTrafficPolicy()).To(BeNil())
		})
	})

	Describe("#GetLoadBalancerServiceProxyProtocolTermination", func() {
		DescribeTable("should return the proxy protocol policy",
			func(allowed bool) {
				seed := &Seed{}
				seed.SetInfo(&gardencorev1beta1.Seed{
					Spec: gardencorev1beta1.SeedSpec{
						Settings: &gardencorev1beta1.SeedSettings{
							LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
								ProxyProtocol: &gardencorev1beta1.LoadBalancerServicesProxyProtocol{
									Allowed: allowed,
								},
							},
						},
					},
				})

				Expect(seed.GetLoadBalancerServiceProxyProtocolTermination()).To(Not(BeNil()))
				Expect(*seed.GetLoadBalancerServiceProxyProtocolTermination()).To(Equal(allowed))
			},

			Entry("proxy protocol is allowed", true),
			Entry("proxy protocol is not allowed", false),
		)

		It("should return no proxy protocol policy if no is available", func() {
			var (
				seed = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{},
					},
				},
			})

			Expect(seed.GetLoadBalancerServiceProxyProtocolTermination()).To(BeNil())
		})

		It("should return no proxy protocol policy if no settings are available", func() {
			var (
				seed = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{}})

			Expect(seed.GetLoadBalancerServiceProxyProtocolTermination()).To(BeNil())
		})
	})

	Describe("#GetZonalLoadBalancerServiceAnnotations", func() {
		It("should return the zonal annotations", func() {
			var (
				annotationKey1   = "my-annotation"
				annotationValue1 = "my-value"
				annotationKey2   = "second-annotation"
				annotationValue2 = "second-value"
				annotationsZone1 = map[string]string{
					annotationKey1: annotationValue1,
					annotationKey2: annotationValue2,
				}
				annotationsZone2 = map[string]string{
					annotationKey1: annotationValue1,
				}

				zone1 = "a"
				zone2 = "b"
				seed  = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
							Zones: []gardencorev1beta1.SeedSettingLoadBalancerServicesZones{
								{
									Name:        zone1,
									Annotations: annotationsZone1,
								},
								{
									Name:        zone2,
									Annotations: annotationsZone2,
								},
							},
						},
					},
				},
			})

			Expect(seed.GetZonalLoadBalancerServiceAnnotations(zone1)).ToNot(ShareSameReferenceAs(annotationsZone1))
			Expect(seed.GetZonalLoadBalancerServiceAnnotations(zone1)).To(Equal(map[string]string{annotationKey1: annotationValue1, annotationKey2: annotationValue2}))
			Expect(seed.GetZonalLoadBalancerServiceAnnotations(zone2)).ToNot(ShareSameReferenceAs(annotationsZone2))
			Expect(seed.GetZonalLoadBalancerServiceAnnotations(zone2)).To(Equal(map[string]string{annotationKey1: annotationValue1}))
		})

		It("should return no annotations if no zonal annotations are available", func() {
			var (
				zone1 = "a"
				seed  = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
							Zones: []gardencorev1beta1.SeedSettingLoadBalancerServicesZones{{
								Name:        zone1,
								Annotations: map[string]string{},
							}},
						},
					},
				},
			})

			Expect(seed.GetZonalLoadBalancerServiceAnnotations(zone1)).To(Equal(map[string]string{}))
		})

		It("should return no zonal annotations if no settings are available", func() {
			var (
				zone1 = "a"
				seed  = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{}})

			Expect(seed.GetZonalLoadBalancerServiceAnnotations(zone1)).To(BeNil())
		})
	})

	Describe("#GetZonalLoadBalancerServiceExternalTrafficPolicy", func() {
		It("should return the zonal traffic policy", func() {
			var (
				policy1 = corev1.ServiceExternalTrafficPolicyLocal
				policy2 = corev1.ServiceExternalTrafficPolicyCluster
				zone1   = "a"
				zone2   = "b"
				seed    = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
							Zones: []gardencorev1beta1.SeedSettingLoadBalancerServicesZones{
								{
									Name:                  zone1,
									ExternalTrafficPolicy: &policy1,
								},
								{
									Name:                  zone2,
									ExternalTrafficPolicy: &policy2,
								},
							},
						},
					},
				},
			})

			Expect(seed.GetZonalLoadBalancerServiceExternalTrafficPolicy(zone1)).To(Equal(&policy1))
			Expect(seed.GetZonalLoadBalancerServiceExternalTrafficPolicy(zone2)).To(Equal(&policy2))
		})

		It("should return no zonal traffic policy if no is available", func() {
			var (
				zone1 = "a"
				seed  = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{},
					},
				},
			})

			Expect(seed.GetZonalLoadBalancerServiceExternalTrafficPolicy(zone1)).To(BeNil())
		})

		It("should return no zonal traffic policy if no settings are available", func() {
			var (
				zone1 = "a"
				seed  = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{}})

			Expect(seed.GetZonalLoadBalancerServiceExternalTrafficPolicy(zone1)).To(BeNil())
		})
	})

	Describe("#GetZonalLoadBalancerServiceProxyProtocolTermination", func() {
		DescribeTable("should return the zonal proxy protocol policy",
			func(allowed1, allowed2 bool) {
				seed := &Seed{}
				zone1 := "a"
				zone2 := "b"
				seed.SetInfo(&gardencorev1beta1.Seed{
					Spec: gardencorev1beta1.SeedSpec{
						Settings: &gardencorev1beta1.SeedSettings{
							LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
								Zones: []gardencorev1beta1.SeedSettingLoadBalancerServicesZones{
									{
										Name: zone1,
										ProxyProtocol: &gardencorev1beta1.LoadBalancerServicesProxyProtocol{
											Allowed: allowed1,
										},
									},
									{
										Name: zone2,
										ProxyProtocol: &gardencorev1beta1.LoadBalancerServicesProxyProtocol{
											Allowed: allowed2,
										},
									},
								},
							},
						},
					},
				})

				Expect(seed.GetZonalLoadBalancerServiceProxyProtocolTermination(zone1)).To(Not(BeNil()))
				Expect(*seed.GetZonalLoadBalancerServiceProxyProtocolTermination(zone1)).To(Equal(allowed1))
				Expect(seed.GetZonalLoadBalancerServiceProxyProtocolTermination(zone2)).To(Not(BeNil()))
				Expect(*seed.GetZonalLoadBalancerServiceProxyProtocolTermination(zone2)).To(Equal(allowed2))
			},

			Entry("proxy protocol is allowed", true, true),
			Entry("proxy protocol is allowed in first zone, but disallowed in second", true, false),
			Entry("proxy protocol is not allowed in first zone, but allowed in second", false, true),
			Entry("proxy protocol is not allowed", false, false),
		)

		It("should return no zonal proxy protocol policy if no is available", func() {
			var (
				zone1 = "a"
				seed  = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{},
					},
				},
			})

			Expect(seed.GetZonalLoadBalancerServiceProxyProtocolTermination(zone1)).To(BeNil())
		})

		It("should return no zonal proxy protocol policy if no settings are available", func() {
			var (
				zone1 = "a"
				seed  = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{}})

			Expect(seed.GetZonalLoadBalancerServiceProxyProtocolTermination(zone1)).To(BeNil())
		})

		It("should return global proxy protocol policy if no zonal settings are available", func() {
			var (
				zone1 = "a"
				seed  = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{
				Settings: &gardencorev1beta1.SeedSettings{
					LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
						ProxyProtocol: &gardencorev1beta1.LoadBalancerServicesProxyProtocol{
							Allowed: true,
						},
					},
				},
			}})

			Expect(seed.GetZonalLoadBalancerServiceProxyProtocolTermination(zone1)).To(Not(BeNil()))
			Expect(*seed.GetZonalLoadBalancerServiceProxyProtocolTermination(zone1)).To(BeTrue())
		})
	})

	Describe("#GetNodeCIDR", func() {
		It("should return the node network CIDR", func() {
			var (
				seed     = &Seed{}
				nodeCIDR = "172.18.0.0/16"
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Networks: gardencorev1beta1.SeedNetworks{
						Nodes: &nodeCIDR,
					},
				},
			})

			Expect(seed.GetNodeCIDR()).To(Equal(&nodeCIDR))
		})
	})
})
