// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/helper"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

var _ = Describe("Helper", func() {
	Describe("#ExtractSeedTemplateAndGardenletConfig", func() {
		var (
			managedSeed *seedmanagementv1alpha1.ManagedSeed
			template    = &gardencorev1beta1.SeedTemplate{
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{
						Type: "foo",
					},
				},
			}
			config = &gardenletconfigv1alpha1.GardenletConfiguration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
					Kind:       "GardenletConfiguration",
				},
				LogLevel: "1234",
			}
		)

		BeforeEach(func() {
			managedSeed = &seedmanagementv1alpha1.ManagedSeed{}
		})

		Context("w/ gardenlet config", func() {
			BeforeEach(func() {
				managedSeed.Spec.Gardenlet = seedmanagementv1alpha1.GardenletConfig{
					Config: runtime.RawExtension{Raw: encode(config)},
				}
			})

			It("should return an error because the gardenlet config cannot be decoded", func() {
				managedSeed.Spec.Gardenlet.Config = runtime.RawExtension{Raw: []byte(`{`)}

				seedTemplate, gardenletConfig, err := ExtractSeedTemplateAndGardenletConfig(managedSeed.Name, &managedSeed.Spec.Gardenlet.Config)
				Expect(seedTemplate).To(BeNil())
				Expect(gardenletConfig).To(BeNil())
				Expect(err).To(MatchError("could not decode gardenlet configuration: couldn't get version/kind; json parse error: unexpected end of JSON input"))
			})

			It("should return an error because seedTemplate is not specified", func() {
				managedSeed.Spec.Gardenlet.Config = runtime.RawExtension{Raw: encode(config)}

				seedTemplate, gardenletConfig, err := ExtractSeedTemplateAndGardenletConfig(managedSeed.Name, &managedSeed.Spec.Gardenlet.Config)
				Expect(seedTemplate).To(BeNil())
				Expect(gardenletConfig).To(BeNil())
				Expect(err).To(HaveOccurred())
			})

			It("should return the template from `.spec.gardenlet.seedConfig.seedTemplate", func() {
				config.SeedConfig = &gardenletconfigv1alpha1.SeedConfig{SeedTemplate: *template}
				managedSeed.Spec.Gardenlet.Config = runtime.RawExtension{Raw: encode(config)}

				seedTemplate, gardenletConfig, err := ExtractSeedTemplateAndGardenletConfig(managedSeed.Name, &managedSeed.Spec.Gardenlet.Config)
				Expect(seedTemplate).To(Equal(template))
				Expect(gardenletConfig).To(Equal(config))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("w/o gardenlet config", func() {
			It("should return an error if seed template cannot be determined", func() {
				seedTemplate, gardenletConfig, err := ExtractSeedTemplateAndGardenletConfig(managedSeed.Name, &managedSeed.Spec.Gardenlet.Config)
				Expect(seedTemplate).To(BeNil())
				Expect(gardenletConfig).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
