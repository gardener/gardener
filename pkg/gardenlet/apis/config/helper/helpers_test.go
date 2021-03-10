// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	corev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	. "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("helper", func() {
	Describe("#SeedNameFromSeedConfig", func() {
		It("should return an empty string", func() {
			Expect(SeedNameFromSeedConfig(nil)).To(BeEmpty())
		})

		It("should return the seed name", func() {
			seedName := "some-name"

			config := &config.SeedConfig{
				SeedTemplate: gardencore.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: seedName,
					},
				},
			}
			Expect(SeedNameFromSeedConfig(config)).To(Equal(seedName))
		})
	})

	Describe("#SeedNames", func() {
		It("should return no name because of missing config and selector", func() {
			Expect(SeedNames(nil, nil, nil)).To(BeEmpty())
		})
		It("should return name of seed config", func() {
			config := &config.SeedConfig{
				SeedTemplate: gardencore.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
			}

			Expect(SeedNames(config, nil, nil)).To(ContainElements(config.SeedTemplate.Name))
		})
		It("should return names matching the seed selector", func() {
			seed1 := &corev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
					Labels: map[string]string{
						"role": "seed",
					},
				},
			}
			seed2 := &corev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bar",
				},
			}

			gardenCoreInformerFactory := gardencoreinformers.NewSharedInformerFactory(nil, 0)
			seedInformer := gardenCoreInformerFactory.Core().V1beta1().Seeds()
			Expect(seedInformer.Informer().GetStore().Add(seed1)).To(Succeed())
			Expect(seedInformer.Informer().GetStore().Add(seed2)).To(Succeed())

			labelSelector, err := metav1.ParseToLabelSelector("role = seed")
			Expect(err).To(Not(HaveOccurred()))

			Expect(SeedNames(nil, seedInformer.Lister(), labelSelector)).To(ContainElements(seed1.Name))
		})
	})

	Describe("#StaleExtensionHealthChecksThreshold", func() {
		It("should return nil when the config is nil", func() {
			Expect(StaleExtensionHealthChecksThreshold(nil)).To(BeNil())
		})

		It("should return nil when the check is not enabled", func() {
			threshold := &metav1.Duration{Duration: time.Minute}
			c := &config.StaleExtensionHealthChecks{
				Enabled:   false,
				Threshold: threshold,
			}
			Expect(StaleExtensionHealthChecksThreshold(c)).To(BeNil())
		})

		It("should return the threshold", func() {
			threshold := &metav1.Duration{Duration: time.Minute}
			c := &config.StaleExtensionHealthChecks{
				Enabled:   true,
				Threshold: threshold,
			}
			Expect(StaleExtensionHealthChecksThreshold(c)).To(Equal(threshold))
		})
	})

	Describe("#ConvertGardenletConfiguration", func() {
		It("should convert the external GardenletConfiguration version to an internal one", func() {
			result, err := ConvertGardenletConfiguration(&configv1alpha1.GardenletConfiguration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: configv1alpha1.SchemeGroupVersion.String(),
					Kind:       "GardenletConfiguration",
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&config.GardenletConfiguration{}))
		})
	})

	Describe("#ConvertGardenletConfigurationExternal", func() {
		It("should convert the internal GardenletConfiguration version to an external one", func() {
			result, err := ConvertGardenletConfigurationExternal(&config.GardenletConfiguration{})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&configv1alpha1.GardenletConfiguration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: configv1alpha1.SchemeGroupVersion.String(),
					Kind:       "GardenletConfiguration",
				},
			}))
		})
	})
})
