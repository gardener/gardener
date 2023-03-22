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

package operation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	. "github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/botanist/component/istio"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("istioconfig", func() {
	DescribeTable("#GetIstioNamespaceForZone",
		func(defaultNamespace string, zone string, matcher gomegatypes.GomegaMatcher) {
			Expect(GetIstioNamespaceForZone(defaultNamespace, zone)).To(matcher)
		},

		Entry("short namespace and zone", "default-namespace", "my-zone", Equal("default-namespace--my-zone")),
		Entry("empty namespace and zone", "", "", Equal("--")),
		Entry("empty namespace and valid zone", "", "my-zone", Equal("--my-zone")),
		Entry("valid namespace and empty zone", "default-namespace", "", Equal("default-namespace--")),
		Entry("namespace and zone too long => hashed zone", "extremely-long-default-namespace", "unnecessarily-long-regional-zone-name", Equal("extremely-long-default-namespace--fc5e9")),
	)

	DescribeTable("#GetIstioZoneLabels",
		func(labels map[string]string, zone *string, matcher gomegatypes.GomegaMatcher) {
			Expect(GetIstioZoneLabels(labels, zone)).To(matcher)
		},

		Entry("no zone, but istio label", map[string]string{istio.DefaultZoneKey: "istio-value"}, nil, Equal(map[string]string{istio.DefaultZoneKey: "istio-value"})),
		Entry("no zone, but gardener.cloud/role label", map[string]string{"gardener.cloud/role": "gardener-role"}, nil, Equal(map[string]string{"gardener.cloud/role": "gardener-role"})),
		Entry("no zone, other labels", map[string]string{"key1": "value1", "key2": "value2"}, nil, Equal(map[string]string{"key1": "value1", "key2": "value2", istio.DefaultZoneKey: "ingressgateway"})),
		Entry("zone and istio label", map[string]string{istio.DefaultZoneKey: "istio-value"}, pointer.String("my-zone"), Equal(map[string]string{istio.DefaultZoneKey: "istio-value--zone--my-zone"})),
		Entry("zone and gardener.cloud/role label", map[string]string{"gardener.cloud/role": "gardener-role"}, pointer.String("my-zone"), Equal(map[string]string{"gardener.cloud/role": "gardener-role--zone--my-zone"})),
		Entry("zone and other labels", map[string]string{"key1": "value1", "key2": "value2"}, pointer.String("my-zone"), Equal(map[string]string{"key1": "value1", "key2": "value2", istio.DefaultZoneKey: "ingressgateway--zone--my-zone"})),
	)

	DescribeTable("#IsZonalIstioExtension",
		func(labels map[string]string, matcherBool gomegatypes.GomegaMatcher, matcherZone gomegatypes.GomegaMatcher) {
			isZone, zone := IsZonalIstioExtension(labels)
			Expect(isZone).To(matcherBool)
			Expect(zone).To(matcherZone)
		},

		Entry("no zonal extension", map[string]string{"key1": "value1", "key2": "value2"}, BeFalse(), Equal("")),
		Entry("no zone, but istio label", map[string]string{istio.DefaultZoneKey: "istio-value"}, BeFalse(), Equal("")),
		Entry("no zone, but gardener.cloud/role label without handler", map[string]string{"gardener.cloud/role": "exposureclass-handler-gardener-role"}, BeFalse(), Equal("")),
		Entry("no zone, but gardener.cloud/role label with handler", map[string]string{"gardener.cloud/role": "exposureclass-handler-gardener-role", "handler.exposureclass.gardener.cloud/name": ""}, BeFalse(), Equal("")),
		Entry("zone and istio label", map[string]string{istio.DefaultZoneKey: "istio-value--zone--my-zone"}, BeTrue(), Equal("my-zone")),
		Entry("zone and gardener.cloud/role label without handler", map[string]string{"gardener.cloud/role": "exposureclass-handler-gardener-role--zone--some-zone"}, BeFalse(), Equal("")),
		Entry("zone and gardener.cloud/role label with handler", map[string]string{"gardener.cloud/role": "exposureclass-handler-gardener-role--zone--some-zone", "handler.exposureclass.gardener.cloud/name": ""}, BeTrue(), Equal("some-zone")),
		Entry("zone and incorrect gardener.cloud/role label with handler", map[string]string{"gardener.cloud/role": "gardener-role--zone--some-zone", "handler.exposureclass.gardener.cloud/name": ""}, BeFalse(), Equal("")),
	)

	Describe("Istio related configuration functions", func() {
		var (
			defaultServiceName         = "default-service"
			defaultNamespaceName       = "default-namespace"
			defaultLabels              = map[string]string{"default": "label", "istio": "gateway"}
			exposureClassHandlerName   = "my-handler"
			exposureClassServiceName   = "exposure-service"
			exposureClassNamespaceName = "exposure-namespace"
			exposureClassLabels        = map[string]string{"exposure": "label"}
			exposureClassAnnotations   = map[string]string{"exposure": "annotation"}
			gardenletConfig            = &config.GardenletConfiguration{
				SNI: &config.SNI{Ingress: &config.SNIIngress{
					ServiceName: &defaultServiceName,
					Namespace:   &defaultNamespaceName,
					Labels:      defaultLabels,
				}},
				ExposureClassHandlers: []config.ExposureClassHandler{
					{
						Name: exposureClassHandlerName,
						LoadBalancerService: config.LoadBalancerServiceConfig{
							Annotations: exposureClassAnnotations,
						},
						SNI: &config.SNI{Ingress: &config.SNIIngress{
							ServiceName: &exposureClassServiceName,
							Namespace:   &exposureClassNamespaceName,
							Labels:      exposureClassLabels,
						}},
					},
				},
			}
			defaultAnnotations = map[string]string{"default": "annotation"}
			zoneName           = "my-zone"
			multiZone          = zoneName + ",other-zone"
			zoneAnnotations    = map[string]string{"zone": "annotation"}
			seed               = &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{
						Zones: []string{zoneName, "some-random-zone"},
					},
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
							Annotations: defaultAnnotations,
							Zones: []gardencorev1beta1.SeedSettingLoadBalancerServicesZones{
								{
									Annotations: zoneAnnotations,
									Name:        zoneName,
								},
							},
						},
					},
				},
			}
			shoot     = &gardencorev1beta1.Shoot{}
			operation = &Operation{
				Config: gardenletConfig,
				Seed:   &seedpkg.Seed{},
				Shoot:  &shootpkg.Shoot{},
				SeedNamespaceObject: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				},
			}
		)

		BeforeEach(func() {
			operation.Seed.SetInfo(seed)
			operation.Shoot.SetInfo(shoot)
		})

		DescribeTable("#component.IstioConfigInterface implementation",
			func(zoneAnnotation *string, useExposureClass bool, matcherService, matcherNamespace, matchLabels, matchAnnotations gomegatypes.GomegaMatcher) {
				if zoneAnnotation != nil {
					operation.SeedNamespaceObject.Annotations[resourcesv1alpha1.HighAvailabilityConfigZones] = *zoneAnnotation
				}
				if useExposureClass {
					shootCopy := shoot.DeepCopy()
					shootCopy.Spec.ExposureClassName = &exposureClassHandlerName
					operation.Shoot.SetInfo(shootCopy)
				}

				Expect(operation.IstioServiceName()).To(matcherService)
				Expect(operation.IstioNamespace()).To(matcherNamespace)
				Expect(operation.IstioLabels()).To(matchLabels)
				Expect(operation.IstioLoadBalancerAnnotations()).To(matchAnnotations)
			},

			Entry("non-pinned control plane without exposure class", nil, false,
				Equal(defaultServiceName),
				Equal(defaultNamespaceName),
				Equal(defaultLabels),
				Equal(defaultAnnotations),
			),
			Entry("pinned control plane (single zone) without exposure class", &zoneName, false,
				Equal(defaultServiceName),
				Equal(defaultNamespaceName+"--"+zoneName),
				Equal(utils.MergeStringMaps(defaultLabels, map[string]string{"istio": defaultLabels["istio"] + "--zone--" + zoneName})),
				Equal(zoneAnnotations),
			),
			Entry("pinned control plane (multi zone) without exposure class", &multiZone, false,
				Equal(defaultServiceName),
				Equal(defaultNamespaceName),
				Equal(defaultLabels),
				Equal(defaultAnnotations),
			),
			Entry("non-pinned control plane with exposure class", nil, true,
				Equal(exposureClassServiceName),
				Equal(exposureClassNamespaceName),
				Equal(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(exposureClassLabels, exposureClassHandlerName)),
				Equal(utils.MergeStringMaps(defaultAnnotations, exposureClassAnnotations)),
			),
			Entry("pinned control plane (single zone) with exposure class", &zoneName, true,
				Equal(exposureClassServiceName),
				Equal(exposureClassNamespaceName+"--"+zoneName),
				Equal(utils.MergeStringMaps(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(exposureClassLabels, exposureClassHandlerName), map[string]string{"gardener.cloud/role": "exposureclass-handler--zone--" + zoneName})),
				Equal(utils.MergeStringMaps(exposureClassAnnotations, zoneAnnotations)),
			),
			Entry("pinned control plane (multi zone) with exposure class", &multiZone, true,
				Equal(exposureClassServiceName),
				Equal(exposureClassNamespaceName),
				Equal(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(exposureClassLabels, exposureClassHandlerName)),
				Equal(utils.MergeStringMaps(defaultAnnotations, exposureClassAnnotations)),
			),
		)

		Context("single-zone seed", func() {
			BeforeEach(func() {
				seedCopy := seed.DeepCopy()
				seedCopy.Spec.Provider.Zones = []string{zoneName}
				operation.Seed.SetInfo(seedCopy)
			})

			DescribeTable("#component.IstioConfigInterface implementation",
				func(zoneAnnotation *string, useExposureClass bool, matcherService, matcherNamespace, matchLabels, matchAnnotations gomegatypes.GomegaMatcher) {
					if zoneAnnotation != nil {
						operation.SeedNamespaceObject.Annotations[resourcesv1alpha1.HighAvailabilityConfigZones] = *zoneAnnotation
					}
					if useExposureClass {
						shootCopy := shoot.DeepCopy()
						shootCopy.Spec.ExposureClassName = &exposureClassHandlerName
						operation.Shoot.SetInfo(shootCopy)
					}

					Expect(operation.IstioServiceName()).To(matcherService)
					Expect(operation.IstioNamespace()).To(matcherNamespace)
					Expect(operation.IstioLabels()).To(matchLabels)
					Expect(operation.IstioLoadBalancerAnnotations()).To(matchAnnotations)
				},

				Entry("pinned control plane (single zone) without exposure class", &zoneName, false,
					Equal(defaultServiceName),
					Equal(defaultNamespaceName),
					Equal(utils.MergeStringMaps(defaultLabels, map[string]string{"istio": defaultLabels["istio"]})),
					Equal(defaultAnnotations),
				),
				Entry("pinned control plane (single zone) with exposure class", &zoneName, true,
					Equal(exposureClassServiceName),
					Equal(exposureClassNamespaceName),
					Equal(utils.MergeStringMaps(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(exposureClassLabels, exposureClassHandlerName), map[string]string{"gardener.cloud/role": "exposureclass-handler"})),
					Equal(utils.MergeStringMaps(defaultAnnotations, exposureClassAnnotations)),
				),
			)
		})
	})
})
