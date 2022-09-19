// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	. "github.com/gardener/gardener/pkg/apis/seedmanagement/helper"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	Describe("#HA Seed Spec tests", func() {
		var (
			managedSeed *seedmanagement.ManagedSeed
			haConfig    gardencore.HighAvailability
		)
		const (
			seedName  = "test-seed"
			namespace = "garden"
			provider  = "test-provider"
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
			haConfig = gardencore.HighAvailability{
				FailureTolerance: gardencore.FailureTolerance{
					Type: gardencore.FailureToleranceTypeZone,
				},
			}
		})

		Context("#ExtractSeedSpec", func() {
			It("seedTemplate is defined", func() {
				managedSeed.Spec.SeedTemplate = &gardencore.SeedTemplate{
					Spec: gardencore.SeedSpec{
						Backup: &gardencore.SeedBackup{
							Provider: provider,
						},
						DNS:              gardencore.SeedDNS{},
						Networks:         gardencore.SeedNetworks{},
						Provider:         gardencore.SeedProvider{},
						HighAvailability: &haConfig,
					},
				}
				spec, err := ExtractSeedSpec(managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(*spec.HighAvailability).To(Equal(haConfig))
			})

			It("gardenlet is defined", func() {
				managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
					Config: &configv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: configv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &configv1alpha1.SeedConfig{
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
				Expect(spec).ToNot(BeNil())
			})

			It("neither seedTemplate nor gardenlet is defined", func() {
				_, err := ExtractSeedSpec(managedSeed)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("#IsMultiZonalManagedSeed", func() {

			It("should return false for non-multi-zonal seeds", func() {
				managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
					Config: &configv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: configv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &configv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								Spec: gardencorev1beta1.SeedSpec{
									Backup: &gardencorev1beta1.SeedBackup{},
								},
							},
						},
					},
				}
				isMultiZonalSeed, err := IsMultiZonalManagedSeed(managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(isMultiZonalSeed).To(BeFalse())
			})

			It("should return true for multi-zonal seed identified via seed spec HA configuration", func() {
				managedSeed.Spec.Gardenlet = &seedmanagement.Gardenlet{
					Config: &configv1alpha1.GardenletConfiguration{
						TypeMeta: metav1.TypeMeta{
							APIVersion: configv1alpha1.SchemeGroupVersion.String(),
							Kind:       "GardenletConfiguration",
						},
						SeedConfig: &configv1alpha1.SeedConfig{
							SeedTemplate: gardencorev1beta1.SeedTemplate{
								Spec: gardencorev1beta1.SeedSpec{
									Backup: &gardencorev1beta1.SeedBackup{},
									HighAvailability: &gardencorev1beta1.HighAvailability{
										FailureTolerance: gardencorev1beta1.FailureTolerance{
											Type: gardencorev1beta1.FailureToleranceTypeZone,
										},
									},
								},
							},
						},
					},
				}
				isMultiZonalSeed, err := IsMultiZonalManagedSeed(managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(isMultiZonalSeed).To(BeTrue())
			})

			It(" should return true for multi-zonal seed identified via multi-zonal label", func() {
				managedSeed.Labels = map[string]string{v1beta1constants.LabelSeedMultiZonal: ""}
				isMultiZonalSeed, err := IsMultiZonalManagedSeed(managedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(isMultiZonalSeed).To(BeTrue())
			})

			It("should return error if no seed spec set", func() {
				_, err := IsMultiZonalManagedSeed(managedSeed)
				Expect(err).To(HaveOccurred())
			})

		})
	})
})

func bootstrapPtr(v seedmanagement.Bootstrap) *seedmanagement.Bootstrap { return &v }
