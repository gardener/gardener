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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	. "github.com/gardener/gardener/pkg/apis/seedmanagement/helper"
	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

var _ = Describe("Helper", func() {
	Describe("#GetBootstrap", func() {
		It("should return the correct Bootstrap value", func() {
			Expect(GetBootstrap(bootstrapPtr(seedmanagement.BootstrapToken))).To(Equal(seedmanagement.BootstrapToken))
			Expect(GetBootstrap(bootstrapPtr(seedmanagement.BootstrapServiceAccount))).To(Equal(seedmanagement.BootstrapServiceAccount))
			Expect(GetBootstrap(bootstrapPtr(seedmanagement.BootstrapNone))).To(Equal(seedmanagement.BootstrapNone))
			Expect(GetBootstrap(nil)).To(Equal(seedmanagement.BootstrapNone))
		})
	})

	Describe("#ExtractSeedSpec", func() {
		var (
			seedName  = "test-seed"
			namespace = "garden"

			managedSeed *seedmanagement.ManagedSeed
		)

		BeforeEach(func() {
			managedSeed = &seedmanagement.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      seedName,
					Namespace: namespace,
				},
				Spec: seedmanagement.ManagedSeedSpec{
					Shoot: &seedmanagement.Shoot{Name: seedName},
				},
			}
		})

		Context("#ExtractSeedSpec", func() {
			It("should extract the seed spec when gardenlet is defined", func() {
				managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
					Config: &gardenletv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								Spec: gardencorev1beta1.SeedSpec{
									Backup: &gardencorev1beta1.SeedBackup{},
								},
							},
						},
					},
				}
				spec, err := ExtractSeedSpec(managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(spec).To(Equal(&gardencore.SeedSpec{
					Backup: &gardencore.SeedBackup{},
				}))
			})

			It("should fail when unsupported gardenlet config is given", func() {
				managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
					Config: &corev1.ConfigMap{},
				}
				_, err := ExtractSeedSpec(managedSeed)
				Expect(err).To(HaveOccurred())
			})

			It("should fail when gardenlet is not defined", func() {
				_, err := ExtractSeedSpec(managedSeed)
				Expect(err).To(HaveOccurred())
			})

			It("should fail when gardenlet config is not defined", func() {
				managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{}

				_, err := ExtractSeedSpec(managedSeed)
				Expect(err).To(HaveOccurred())
			})

			It("should fail when seedConfig is not defined in gardenlet config", func() {
				managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
					Config: &gardenletv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
					},
				}

				_, err := ExtractSeedSpec(managedSeed)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

func bootstrapPtr(v seedmanagement.Bootstrap) *seedmanagement.Bootstrap { return &v }
