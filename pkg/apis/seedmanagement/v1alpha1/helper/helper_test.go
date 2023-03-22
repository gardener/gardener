// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

var _ = Describe("Helper", func() {
	Describe("#GetBootstrap", func() {
		It("should return the correct Bootstrap value", func() {
			Expect(GetBootstrap(bootstrapPtr(seedmanagementv1alpha1.BootstrapToken))).To(Equal(seedmanagementv1alpha1.BootstrapToken))
			Expect(GetBootstrap(bootstrapPtr(seedmanagementv1alpha1.BootstrapServiceAccount))).To(Equal(seedmanagementv1alpha1.BootstrapServiceAccount))
			Expect(GetBootstrap(bootstrapPtr(seedmanagementv1alpha1.BootstrapNone))).To(Equal(seedmanagementv1alpha1.BootstrapNone))
			Expect(GetBootstrap(nil)).To(Equal(seedmanagementv1alpha1.BootstrapNone))
		})
	})

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
			config = &gardenletv1alpha1.GardenletConfiguration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
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
				managedSeed.Spec.Gardenlet = &seedmanagementv1alpha1.Gardenlet{
					Config: runtime.RawExtension{Raw: encode(config)},
				}
			})

			It("should return an error because the gardenlet config cannot be decoded", func() {
				managedSeed.Spec.Gardenlet.Config = runtime.RawExtension{Raw: []byte(`{`)}

				seedTemplate, gardenletConfig, err := ExtractSeedTemplateAndGardenletConfig(managedSeed)
				Expect(seedTemplate).To(BeNil())
				Expect(gardenletConfig).To(BeNil())
				Expect(err).To(MatchError("could not decode gardenlet configuration: couldn't get version/kind; json parse error: unexpected end of JSON input"))
			})

			It("should return an error because seedTemplate is not specified", func() {
				managedSeed.Spec.Gardenlet.Config = runtime.RawExtension{Raw: encode(config)}

				seedTemplate, gardenletConfig, err := ExtractSeedTemplateAndGardenletConfig(managedSeed)
				Expect(seedTemplate).To(BeNil())
				Expect(gardenletConfig).To(BeNil())
				Expect(err).To(HaveOccurred())
			})

			It("should return the template from `.spec.gardenlet.seedConfig.seedTemplate", func() {
				config.SeedConfig = &gardenletv1alpha1.SeedConfig{SeedTemplate: *template}
				managedSeed.Spec.Gardenlet.Config = runtime.RawExtension{Raw: encode(config)}

				seedTemplate, gardenletConfig, err := ExtractSeedTemplateAndGardenletConfig(managedSeed)
				Expect(seedTemplate).To(Equal(template))
				Expect(gardenletConfig).To(Equal(config))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("w/o gardenlet config", func() {
			It("should return an error if seed template cannot be determined", func() {
				seedTemplate, gardenletConfig, err := ExtractSeedTemplateAndGardenletConfig(managedSeed)
				Expect(seedTemplate).To(BeNil())
				Expect(gardenletConfig).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

func bootstrapPtr(v seedmanagementv1alpha1.Bootstrap) *seedmanagementv1alpha1.Bootstrap { return &v }

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
