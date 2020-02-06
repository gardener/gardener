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

package validation_test

import (
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("CloudProfile Validation Tests ", func() {
	Describe("#ValidateCloudProfile", func() {
		var (
			metadata = metav1.ObjectMeta{
				Name: "profile",
			}
			machineType = core.MachineType{
				Name:   "machine-type-1",
				CPU:    resource.MustParse("2"),
				GPU:    resource.MustParse("0"),
				Memory: resource.MustParse("100Gi"),
			}
			machineTypesConstraint = []core.MachineType{
				machineType,
			}
			volumeTypesConstraint = []core.VolumeType{
				{
					Name:  "volume-type-1",
					Class: "super-premium",
				},
			}

			invalidMachineType = core.MachineType{
				Name:   "",
				CPU:    resource.MustParse("-1"),
				GPU:    resource.MustParse("-1"),
				Memory: resource.MustParse("-100Gi"),
			}
			invalidMachineTypes = []core.MachineType{
				invalidMachineType,
			}
			invalidVolumeTypes = []core.VolumeType{
				{
					Name:  "",
					Class: "",
				},
			}
		)

		It("should forbid empty CloudProfile resources", func() {
			cloudProfile := &core.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       core.CloudProfileSpec{},
			}

			errorList := ValidateCloudProfile(cloudProfile)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.type"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.kubernetes.versions"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.machineImages"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.machineTypes"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.regions"),
				}))))
		})

		Context("tests for unknown cloud profiles", func() {
			var (
				regionName = "region1"
				zoneName   = "zone1"

				unknownCloudProfile *core.CloudProfile

				duplicatedKubernetes = core.KubernetesSettings{
					Versions: []core.ExpirableVersion{{Version: "1.11.4"}, {Version: "1.11.4"}},
				}
				duplicatedRegions = []core.Region{
					{
						Name: regionName,
						Zones: []core.AvailabilityZone{
							{Name: zoneName},
						},
					},
					{
						Name: regionName,
						Zones: []core.AvailabilityZone{
							{Name: zoneName},
						},
					},
				}
				duplicatedZones = []core.Region{
					{
						Name: regionName,
						Zones: []core.AvailabilityZone{
							{Name: zoneName},
							{Name: zoneName},
						},
					},
				}
			)

			BeforeEach(func() {
				unknownCloudProfile = &core.CloudProfile{
					ObjectMeta: metadata,
					Spec: core.CloudProfileSpec{
						Type: "unknown",
						SeedSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
						Kubernetes: core.KubernetesSettings{
							Versions: []core.ExpirableVersion{{Version: "1.11.4"}},
						},
						MachineImages: []core.MachineImage{
							{
								Name: "some-machineimage",
								Versions: []core.ExpirableVersion{
									{Version: "1.2.3"},
								},
							},
						},
						Regions: []core.Region{
							{
								Name: regionName,
								Zones: []core.AvailabilityZone{
									{Name: zoneName},
								},
							},
						},
						MachineTypes: machineTypesConstraint,
						VolumeTypes:  volumeTypesConstraint,
					},
				}
			})

			It("should not return any errors", func() {
				errorList := ValidateCloudProfile(unknownCloudProfile)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid not specifying a type", func() {
				unknownCloudProfile.Spec.Type = ""

				errorList := ValidateCloudProfile(unknownCloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.type"),
				}))))
			})

			It("should forbid ca bundles with unsupported format", func() {
				unknownCloudProfile.Spec.CABundle = makeStringPointer("unsupported")

				errorList := ValidateCloudProfile(unknownCloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.caBundle"),
				}))))
			})

			Context("kubernetes version constraints", func() {
				It("should enforce that at least one version has been defined", func() {
					unknownCloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.kubernetes.versions"),
					}))))
				})

				It("should forbid versions of a not allowed pattern", func() {
					unknownCloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{{Version: "1.11"}}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.versions[0]"),
					}))))
				})

				It("should forbid expiration date on latest kubernetes version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					unknownCloudProfile.Spec.Kubernetes.Versions = []core.ExpirableVersion{
						{
							Version: "1.1.0",
						},
						{
							Version:        "1.2.0",
							ExpirationDate: expirationDate,
						},
					}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.versions[].expirationDate"),
					}))))
				})

				It("should forbid duplicated kubernetes versions", func() {
					unknownCloudProfile.Spec.Kubernetes = duplicatedKubernetes

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.kubernetes.versions[%d].version", len(duplicatedKubernetes.Versions)-1)),
						}))))
				})
			})

			Context("machine image validation", func() {
				It("should forbid an empty list of machine images", func() {
					unknownCloudProfile.Spec.MachineImages = []core.MachineImage{}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineImages"),
					}))))
				})

				It("should forbid duplicate names in list of machine images", func() {
					unknownCloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []core.ExpirableVersion{
								{Version: "3.4.6"},
							},
						},
						{
							Name: "some-machineimage",
							Versions: []core.ExpirableVersion{
								{Version: "3.4.5"},
							},
						},
					}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.machineImages[1]"),
					}))))
				})

				It("should forbid machine images with no version", func() {
					unknownCloudProfile.Spec.MachineImages = []core.MachineImage{
						{Name: "some-machineimage"},
					}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineImages[0].versions"),
					}))))
				})

				It("should forbid nonSemVer machine image versions", func() {
					unknownCloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []core.ExpirableVersion{
								{
									Version: "0.1.2"},
							},
						},
						{
							Name: "xy",
							Versions: []core.ExpirableVersion{
								{
									Version: "a.b.c",
								},
							},
						},
					}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineImages"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineImages[1].versions[0].version"),
					}))))
				})

				It("should forbid expiration date on latest machine image version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					unknownCloudProfile.Spec.MachineImages = []core.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []core.ExpirableVersion{
								{
									Version:        "0.1.2",
									ExpirationDate: expirationDate,
								},
								{
									Version: "0.1.1",
								},
							},
						},
						{
							Name: "xy",
							Versions: []core.ExpirableVersion{
								{
									Version:        "0.1.1",
									ExpirationDate: expirationDate,
								},
							},
						},
					}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.machineImages.expirationDate"),
						"Detail": ContainSubstring("some-machineimage"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.machineImages.expirationDate"),
						"Detail": ContainSubstring("xy"),
					}))))
				})
			})

			Context("machine types validation", func() {
				It("should enforce that at least one machine type has been defined", func() {
					unknownCloudProfile.Spec.MachineTypes = []core.MachineType{}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineTypes"),
					}))))
				})

				It("should enforce uniqueness of machine type names", func() {
					unknownCloudProfile.Spec.MachineTypes = []core.MachineType{
						unknownCloudProfile.Spec.MachineTypes[0],
						unknownCloudProfile.Spec.MachineTypes[0],
					}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.machineTypes[1].name"),
					}))))
				})

				It("should forbid machine types with unsupported property values", func() {
					unknownCloudProfile.Spec.MachineTypes = invalidMachineTypes

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineTypes[0].name"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[0].cpu"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[0].gpu"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.machineTypes[0].memory"),
					}))))
				})
			})

			Context("regions validation", func() {
				It("should forbid regions with unsupported name values", func() {
					unknownCloudProfile.Spec.Regions = []core.Region{
						{
							Name:  "",
							Zones: []core.AvailabilityZone{{Name: ""}},
						},
					}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.regions[0].name"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.regions[0].zones[0].name"),
					}))))
				})

				It("should forbid duplicated region names", func() {
					unknownCloudProfile.Spec.Regions = duplicatedRegions

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.regions[%d].name", len(duplicatedRegions)-1)),
						}))))
				})

				It("should forbid duplicated zone names", func() {
					unknownCloudProfile.Spec.Regions = duplicatedZones

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.regions[0].zones[%d].name", len(duplicatedZones[0].Zones)-1)),
						}))))
				})
			})

			Context("volume types validation", func() {
				It("should enforce uniqueness of volume type names", func() {
					unknownCloudProfile.Spec.VolumeTypes = []core.VolumeType{
						unknownCloudProfile.Spec.VolumeTypes[0],
						unknownCloudProfile.Spec.VolumeTypes[0],
					}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.volumeTypes[1].name"),
					}))))
				})

				It("should forbid volume types with unsupported property values", func() {
					unknownCloudProfile.Spec.VolumeTypes = invalidVolumeTypes

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.volumeTypes[0].name"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.volumeTypes[0].class"),
					}))))
				})
			})

			It("should forbid unsupported seed selectors", func() {
				unknownCloudProfile.Spec.SeedSelector.MatchLabels["foo"] = "no/slash/allowed"

				errorList := ValidateCloudProfile(unknownCloudProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.seedSelector.matchLabels"),
				}))))
			})
		})
	})
})
