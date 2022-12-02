// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/operation/seed"
)

var _ = Describe("seed", func() {
	Describe("#GetValidVolumeSize", func() {
		It("should return the size because no minimum size was set", func() {
			var (
				size = "20Gi"
				seed = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Volume: nil,
				},
			})

			Expect(seed.GetValidVolumeSize(size)).To(Equal(size))
		})

		It("should return the minimum size because the given value is smaller", func() {
			var (
				size                = "20Gi"
				minimumSize         = "25Gi"
				minimumSizeQuantity = resource.MustParse(minimumSize)
				seed                = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Volume: &gardencorev1beta1.SeedVolume{
						MinimumSize: &minimumSizeQuantity,
					},
				},
			})

			Expect(seed.GetValidVolumeSize(size)).To(Equal(minimumSize))
		})

		It("should return the given value size because the minimum size is smaller", func() {
			var (
				size                = "30Gi"
				minimumSize         = "25Gi"
				minimumSizeQuantity = resource.MustParse(minimumSize)
				seed                = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Volume: &gardencorev1beta1.SeedVolume{
						MinimumSize: &minimumSizeQuantity,
					},
				},
			})

			Expect(seed.GetValidVolumeSize(size)).To(Equal(size))
		})
	})

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

			Expect(seed.GetLoadBalancerServiceAnnotations()).To(Equal(map[string]string{annotationKey1: annotationValue1, annotationKey2: annotationValue2}))
		})

		It("should return no annotations if no annoations are available", func() {
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

			Expect(seed.GetLoadBalancerServiceAnnotations()).To(Equal(map[string]string{}))
		})

		It("should return no annotations if no settings are available", func() {
			var (
				seed = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{}})

			Expect(seed.GetLoadBalancerServiceAnnotations()).To(Equal(map[string]string{}))
		})
	})

	Describe("#GetLoadBalancerServiceExternalTrafficPolicy", func() {
		It("should return the traffic policy", func() {
			var (
				policy = corev1.ServiceExternalTrafficPolicyTypeLocal
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

	Describe("#GetZonalLoadBalancerServiceAnnotations", func() {
		It("should return the zonal annotations", func() {
			var (
				annotationKey1   = "my-annotation"
				annotationValue1 = "my-value"
				annotationKey2   = "second-annotation"
				annotationValue2 = "second-value"
				zone1            = "a"
				zone2            = "b"
				seed             = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
							ZoneSettings: []gardencorev1beta1.SeedSettingLoadBalancerServicesZoneSettings{
								{
									ZoneName: zone1,
									Annotations: map[string]string{
										annotationKey1: annotationValue1,
										annotationKey2: annotationValue2,
									},
								},
								{
									ZoneName: zone2,
									Annotations: map[string]string{
										annotationKey1: annotationValue1,
									},
								},
							},
						},
					},
				},
			})

			Expect(seed.GetZonalLoadBalancerServiceAnnotations(zone1)).To(Equal(map[string]string{annotationKey1: annotationValue1, annotationKey2: annotationValue2}))
			Expect(seed.GetZonalLoadBalancerServiceAnnotations(zone2)).To(Equal(map[string]string{annotationKey1: annotationValue1}))
		})

		It("should return no annotations if no zonal annoations are available", func() {
			var (
				zone1 = "a"
				seed  = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
							ZoneSettings: []gardencorev1beta1.SeedSettingLoadBalancerServicesZoneSettings{{
								ZoneName:    zone1,
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

			Expect(seed.GetZonalLoadBalancerServiceAnnotations(zone1)).To(Equal(map[string]string{}))
		})
	})

	Describe("#GetZonalLoadBalancerServiceExternalTrafficPolicy", func() {
		It("should return the zonal traffic policy", func() {
			var (
				policy1 = corev1.ServiceExternalTrafficPolicyTypeLocal
				policy2 = corev1.ServiceExternalTrafficPolicyTypeCluster
				zone1   = "a"
				zone2   = "b"
				seed    = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
							ZoneSettings: []gardencorev1beta1.SeedSettingLoadBalancerServicesZoneSettings{
								{
									ZoneName:              zone1,
									ExternalTrafficPolicy: &policy1,
								},
								{
									ZoneName:              zone2,
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
})
