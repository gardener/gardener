// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/operation"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("istioconfig", func() {
	Describe("Istio related configuration functions", func() {
		var (
			defaultServiceName         = "default-service"
			defaultNamespaceName       = "default-namespace"
			defaultLabels              = map[string]string{"default": "label", "istio": "gateway"}
			exposureClassName          = "my-exposureclass"
			exposureClassHandlerName   = "my-handler"
			exposureClassServiceName   = "exposure-service"
			exposureClassNamespaceName = "exposure-namespace"
			exposureClassLabels        = map[string]string{"exposure": "label"}
			gardenletConfig            = &gardenletconfigv1alpha1.GardenletConfiguration{
				SNI: &gardenletconfigv1alpha1.SNI{Ingress: &gardenletconfigv1alpha1.SNIIngress{
					ServiceName: &defaultServiceName,
					Namespace:   &defaultNamespaceName,
					Labels:      defaultLabels,
				}},
				ExposureClassHandlers: []gardenletconfigv1alpha1.ExposureClassHandler{
					{
						Name: exposureClassHandlerName,
						SNI: &gardenletconfigv1alpha1.SNI{Ingress: &gardenletconfigv1alpha1.SNIIngress{
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

			operation     *Operation
			exposureClass *gardencorev1beta1.ExposureClass
			shoot         *gardencorev1beta1.Shoot
			seed          *gardencorev1beta1.Seed
		)

		BeforeEach(func() {
			seed = &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{
						Zones: []string{zoneName, "some-random-zone"},
					},
					Settings: &gardencorev1beta1.SeedSettings{
						LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
							Annotations: defaultAnnotations,
							Zones: []gardencorev1beta1.SeedSettingLoadBalancerServicesZones{
								{
									Name: zoneName,
								},
							},
						},
					},
				},
			}
			exposureClass = &gardencorev1beta1.ExposureClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: exposureClassName,
				},
				Handler: exposureClassHandlerName,
			}
			shoot = &gardencorev1beta1.Shoot{}
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

			operation.Seed.SetInfo(seed)
			operation.Shoot.SetInfo(shoot)
		})

		DescribeTable("#component.IstioConfigInterface implementation",
			func(zoneAnnotation *string, useExposureClass bool, matcherService, matcherNamespace, matchLabels gomegatypes.GomegaMatcher) {
				if zoneAnnotation != nil {
					operation.SeedNamespaceObject.Annotations[resourcesv1alpha1.HighAvailabilityConfigZones] = *zoneAnnotation
				}
				if useExposureClass {
					shootCopy := shoot.DeepCopy()
					shootCopy.Spec.ExposureClassName = &exposureClassName
					operation.Shoot.SetInfo(shootCopy)
					operation.Shoot.ExposureClass = exposureClass
				}

				Expect(operation.IstioServiceName()).To(matcherService)
				Expect(operation.IstioNamespace()).To(matcherNamespace)
				Expect(operation.IstioLabels()).To(matchLabels)
			},

			Entry("non-pinned control plane without exposure class", nil, false,
				Equal(defaultServiceName),
				Equal(defaultNamespaceName),
				Equal(defaultLabels),
			),
			Entry("pinned control plane (single zone) without exposure class", &zoneName, false,
				Equal(defaultServiceName),
				Equal(defaultNamespaceName+"--"+zoneName),
				Equal(utils.MergeStringMaps(defaultLabels, map[string]string{"istio": defaultLabels["istio"] + "--zone--" + zoneName})),
			),
			Entry("pinned control plane (multi zone) without exposure class", &multiZone, false,
				Equal(defaultServiceName),
				Equal(defaultNamespaceName),
				Equal(defaultLabels),
			),
			Entry("non-pinned control plane with exposure class", nil, true,
				Equal(exposureClassServiceName),
				Equal(exposureClassNamespaceName),
				Equal(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(exposureClassLabels, exposureClassHandlerName)),
			),
			Entry("pinned control plane (single zone) with exposure class", &zoneName, true,
				Equal(exposureClassServiceName),
				Equal(exposureClassNamespaceName+"--"+zoneName),
				Equal(utils.MergeStringMaps(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(exposureClassLabels, exposureClassHandlerName), map[string]string{"gardener.cloud/role": "exposureclass-handler--zone--" + zoneName})),
			),
			Entry("pinned control plane (multi zone) with exposure class", &multiZone, true,
				Equal(exposureClassServiceName),
				Equal(exposureClassNamespaceName),
				Equal(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(exposureClassLabels, exposureClassHandlerName)),
			),
		)

		Context("single-zone seed", func() {
			BeforeEach(func() {
				seedCopy := seed.DeepCopy()
				seedCopy.Spec.Provider.Zones = []string{zoneName}
				operation.Seed.SetInfo(seedCopy)
			})

			DescribeTable("#component.IstioConfigInterface implementation",
				func(zoneAnnotation *string, useExposureClass bool, matcherService, matcherNamespace, matchLabels gomegatypes.GomegaMatcher) {
					if zoneAnnotation != nil {
						operation.SeedNamespaceObject.Annotations[resourcesv1alpha1.HighAvailabilityConfigZones] = *zoneAnnotation
					}
					if useExposureClass {
						shootCopy := shoot.DeepCopy()
						shootCopy.Spec.ExposureClassName = &exposureClassName
						operation.Shoot.SetInfo(shootCopy)
						operation.Shoot.ExposureClass = exposureClass
					}

					Expect(operation.IstioServiceName()).To(matcherService)
					Expect(operation.IstioNamespace()).To(matcherNamespace)
					Expect(operation.IstioLabels()).To(matchLabels)
				},

				Entry("pinned control plane (single zone) without exposure class", &zoneName, false,
					Equal(defaultServiceName),
					Equal(defaultNamespaceName),
					Equal(utils.MergeStringMaps(defaultLabels, map[string]string{"istio": defaultLabels["istio"]})),
				),
				Entry("pinned control plane (single zone) with exposure class", &zoneName, true,
					Equal(exposureClassServiceName),
					Equal(exposureClassNamespaceName),
					Equal(utils.MergeStringMaps(gardenerutils.GetMandatoryExposureClassHandlerSNILabels(exposureClassLabels, exposureClassHandlerName), map[string]string{"gardener.cloud/role": "exposureclass-handler"})),
				),
			)
		})
	})
})
