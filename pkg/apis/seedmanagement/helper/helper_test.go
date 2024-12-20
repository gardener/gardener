// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

var _ = Describe("Helper", func() {
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
				managedSeed.Spec.Gardenlet = seedmanagement.GardenletConfig{
					Config: &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
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
				managedSeed.Spec.Gardenlet = seedmanagement.GardenletConfig{
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
				managedSeed.Spec.Gardenlet = seedmanagement.GardenletConfig{}

				_, err := ExtractSeedSpec(managedSeed)
				Expect(err).To(HaveOccurred())
			})

			It("should fail when seedConfig is not defined in gardenlet config", func() {
				managedSeed.Spec.Gardenlet = seedmanagement.GardenletConfig{
					Config: &gardenletconfigv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
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
