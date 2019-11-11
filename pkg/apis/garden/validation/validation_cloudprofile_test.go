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
	"github.com/gardener/gardener/pkg/apis/garden"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"time"

	. "github.com/gardener/gardener/pkg/apis/garden/validation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("CloudProfile Validation Tests ", func() {
	Describe("#ValidateCloudProfile", func() {
		var (
			metadata = metav1.ObjectMeta{
				Name: "profile",
			}
			kubernetesVersionConstraint = garden.KubernetesConstraints{
				OfferedVersions: []garden.KubernetesVersion{{Version: "1.11.4"}},
			}
			machineType = garden.MachineType{
				Name:   "machine-type-1",
				CPU:    resource.MustParse("2"),
				GPU:    resource.MustParse("0"),
				Memory: resource.MustParse("100Gi"),
			}
			machineTypesConstraint = []garden.MachineType{
				machineType,
			}
			openStackMachineTypesConstraint = []garden.OpenStackMachineType{
				{
					MachineType: machineType,
					VolumeType:  "default",
					VolumeSize:  resource.MustParse("20Gi"),
				},
			}
			volumeTypesConstraint = []garden.VolumeType{
				{
					Name:  "volume-type-1",
					Class: "super-premium",
				},
			}
			zonesConstraint = []garden.Zone{
				{
					Region: "my-region-",
					Names:  []string{"my-region-a"},
				},
			}

			invalidKubernetes    = []garden.KubernetesVersion{{Version: "1.11"}}
			duplicatedKubernetes = []garden.KubernetesVersion{{Version: "1.11.4"}, {Version: "1.11.4"}}
			invalidMachineType   = garden.MachineType{
				Name:   "",
				CPU:    resource.MustParse("-1"),
				GPU:    resource.MustParse("-1"),
				Memory: resource.MustParse("-100Gi"),
			}
			invalidMachineTypes = []garden.MachineType{
				invalidMachineType,
			}
			invalidOpenStackMachineTypes = []garden.OpenStackMachineType{
				{
					MachineType: invalidMachineType,
					VolumeType:  "",
					VolumeSize:  resource.MustParse("-100Gi"),
				},
			}
			invalidVolumeTypes = []garden.VolumeType{
				{
					Name:  "",
					Class: "",
				},
			}
			invalidZones = []garden.Zone{
				{
					Region: "",
					Names:  []string{""},
				},
			}
			duplicatedRegionsConstraint = []garden.Zone{
				{
					Region: "my-region-",
					Names:  []string{"my-region-a"},
				},
				{
					Region: "my-region-",
					Names:  []string{"my-region-a"},
				},
			}
			duplicatedZonesConstraint = []garden.Zone{
				{
					Region: "my-region-",
					Names:  []string{"my-region-a", "my-region-a"},
				},
			}
		)

		It("should forbid empty CloudProfile resources", func() {
			cloudProfile := &garden.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       garden.CloudProfileSpec{},
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
				}))))
		})

		Context("tests for AWS cloud profiles", func() {
			var (
				fldPath         = "aws"
				awsCloudProfile *garden.CloudProfile
			)

			BeforeEach(func() {
				awsCloudProfile = &garden.CloudProfile{
					ObjectMeta: metadata,
					Spec: garden.CloudProfileSpec{
						AWS: &garden.AWSProfile{
							Constraints: garden.AWSConstraints{
								Kubernetes: kubernetesVersionConstraint,
								MachineImages: []garden.MachineImage{
									{
										Name: "some-machineimage",
										Versions: []garden.MachineImageVersion{
											{
												Version: "1.2.3"},
										},
									},
								},
								MachineTypes: machineTypesConstraint,
								VolumeTypes:  volumeTypesConstraint,
								Zones:        zonesConstraint,
							},
						},
						Type: "aws",
						Kubernetes: garden.KubernetesSettings{
							Versions: []garden.ExpirableVersion{{Version: "1.11.4"}},
						},
						MachineImages: []garden.CloudProfileMachineImage{
							{
								Name: "some-machineimage",
								Versions: []garden.ExpirableVersion{
									{Version: "1.2.3"},
								},
							},
						},
						MachineTypes: machineTypesConstraint,
					},
				}
			})

			It("should not return any errors", func() {
				errorList := ValidateCloudProfile(awsCloudProfile)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid ca bundles with unsupported format", func() {
				awsCloudProfile.Spec.CABundle = makeStringPointer("unsupported")

				errorList := ValidateCloudProfile(awsCloudProfile)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.caBundle"),
				}))
			})

			Context("kubernetes version constraints", func() {
				It("should enforce that at least one version has been defined", func() {
					awsCloudProfile.Spec.AWS.Constraints.Kubernetes.OfferedVersions = []garden.KubernetesVersion{}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions", fldPath)),
					}))
				})

				It("should forbid versions of a not allowed pattern", func() {
					awsCloudProfile.Spec.AWS.Constraints.Kubernetes.OfferedVersions = invalidKubernetes

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[0]", fldPath)),
					}))
				})

				It("should forbid expiration date on latest kubernetes version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					awsCloudProfile.Spec.AWS.Constraints.Kubernetes.OfferedVersions = []garden.KubernetesVersion{
						{
							Version: "1.1.0",
						},
						{
							Version:        "1.2.0",
							ExpirationDate: expirationDate,
						},
					}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[].expirationDate", fldPath)),
					}))
				})

				It("should forbid duplicated kubernetes versions", func() {
					awsCloudProfile.Spec.AWS.Constraints.Kubernetes.OfferedVersions = duplicatedKubernetes

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[%d].version", fldPath, len(duplicatedKubernetes)-1)),
						}))))
				})
			})

			Context("machine image validation", func() {
				It("should forbid an empty list of machine images", func() {
					awsCloudProfile.Spec.AWS.Constraints.MachineImages = []garden.MachineImage{}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages", fldPath)),
					}))
				})

				It("should forbid duplicate names in list of machine images", func() {
					awsCloudProfile.Spec.AWS.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
								{
									Version: "3.4.6"},
							},
						},
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
								{
									Version: "3.4.5",
								},
							},
						},
					}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[1]", fldPath)),
					}))
				})

				It("should forbid machine images with no version", func() {
					awsCloudProfile.Spec.AWS.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
						},
					}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[0].versions", fldPath)),
					}))))
				})

				It("should forbid nonSemVer machine image versions", func() {
					awsCloudProfile.Spec.AWS.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
								{
									Version: "0.1.2"},
							},
						},
						{
							Name: "xy",
							Versions: []garden.MachineImageVersion{
								{
									Version: "a.b.c",
								},
							},
						},
					}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages", fldPath)),
					})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[1].versions[0].version", fldPath)),
						}))))
				})
				It("should forbid expiration date on latest machine image version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					awsCloudProfile.Spec.AWS.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
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
							Versions: []garden.MachineImageVersion{
								{
									Version:        "0.1.1",
									ExpirationDate: expirationDate,
								},
							},
						},
					}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(fmt.Sprintf("spec.%s.constraints.machineImages.expirationDate", fldPath)),
						"Detail": ContainSubstring("some-machineimage"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(fmt.Sprintf("spec.%s.constraints.machineImages.expirationDate", fldPath)),
						"Detail": ContainSubstring("xy"),
					}))))
				})
			})

			Context("machine types validation", func() {
				It("should enforce that at least one machine type has been defined", func() {
					awsCloudProfile.Spec.AWS.Constraints.MachineTypes = []garden.MachineType{}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes", fldPath)),
					}))
				})

				It("should enforce uniqueness of machine type names", func() {
					awsCloudProfile.Spec.AWS.Constraints.MachineTypes = []garden.MachineType{
						awsCloudProfile.Spec.AWS.Constraints.MachineTypes[0],
						awsCloudProfile.Spec.AWS.Constraints.MachineTypes[0],
					}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[1].name", fldPath)),
					}))))
				})

				It("should forbid machine types with unsupported property values", func() {
					awsCloudProfile.Spec.AWS.Constraints.MachineTypes = invalidMachineTypes

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(HaveLen(4))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].name", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].cpu", fldPath)),
					}))
					Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].gpu", fldPath)),
					}))
					Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].memory", fldPath)),
					}))
				})
			})

			Context("volume types validation", func() {
				It("should enforce uniqueness of volume type names", func() {
					awsCloudProfile.Spec.AWS.Constraints.VolumeTypes = []garden.VolumeType{
						awsCloudProfile.Spec.AWS.Constraints.VolumeTypes[0],
						awsCloudProfile.Spec.AWS.Constraints.VolumeTypes[0],
					}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[1].name", fldPath)),
					}))))
				})

				It("should forbid volume types with unsupported property values", func() {
					awsCloudProfile.Spec.AWS.Constraints.VolumeTypes = invalidVolumeTypes

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[0].name", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[0].class", fldPath)),
					}))
				})
			})

			Context("zones validation", func() {
				It("should enforce that at least one zone has been defined", func() {
					awsCloudProfile.Spec.AWS.Constraints.Zones = []garden.Zone{}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones", fldPath)),
					}))
				})

				It("should forbid zones with unsupported name values", func() {
					awsCloudProfile.Spec.AWS.Constraints.Zones = invalidZones

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].region", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].names[0]", fldPath)),
					}))
				})

				It("should forbid duplicated region names", func() {
					awsCloudProfile.Spec.AWS.Constraints.Zones = duplicatedRegionsConstraint

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[%d].region", fldPath, len(duplicatedRegionsConstraint)-1)),
						}))))
				})

				It("should forbid duplicated zone names", func() {
					awsCloudProfile.Spec.AWS.Constraints.Zones = duplicatedZonesConstraint

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].names[%d]", fldPath, len(duplicatedZonesConstraint[0].Names)-1)),
						}))))
				})
			})
		})

		Context("tests for Azure cloud profiles", func() {
			var (
				fldPath           = "azure"
				azureCloudProfile *garden.CloudProfile
			)

			BeforeEach(func() {
				azureCloudProfile = &garden.CloudProfile{
					ObjectMeta: metadata,
					Spec: garden.CloudProfileSpec{
						Azure: &garden.AzureProfile{
							Constraints: garden.AzureConstraints{
								Kubernetes: kubernetesVersionConstraint,
								MachineImages: []garden.MachineImage{
									{
										Name:     "some-machineimage",
										Versions: []garden.MachineImageVersion{{Version: "1.6.4"}},
									},
								},
								MachineTypes: machineTypesConstraint,
								VolumeTypes:  volumeTypesConstraint,
							},
							CountFaultDomains: []garden.AzureDomainCount{
								{
									Region: "westeurope",
									Count:  0,
								},
							},
							CountUpdateDomains: []garden.AzureDomainCount{
								{
									Region: "westeurope",
									Count:  0,
								},
							},
						},
						Type: "azure",
						Kubernetes: garden.KubernetesSettings{
							Versions: []garden.ExpirableVersion{{Version: "1.11.4"}},
						},
						MachineImages: []garden.CloudProfileMachineImage{
							{
								Name: "some-machineimage",
								Versions: []garden.ExpirableVersion{
									{Version: "1.2.3"},
								},
							},
						},
						MachineTypes: machineTypesConstraint,
					},
				}
			})

			Context("kubernetes version constraints", func() {
				It("should enforce that at least one version has been defined", func() {
					azureCloudProfile.Spec.Azure.Constraints.Kubernetes.OfferedVersions = []garden.KubernetesVersion{}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions", fldPath)),
					}))
				})

				It("should forbid versions of a not allowed pattern", func() {
					azureCloudProfile.Spec.Azure.Constraints.Kubernetes.OfferedVersions = invalidKubernetes

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[0]", fldPath)),
					}))
				})

				It("should forbid expiration date on latest kubernetes version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					azureCloudProfile.Spec.Azure.Constraints.Kubernetes.OfferedVersions = []garden.KubernetesVersion{
						{
							Version: "1.1.0",
						},
						{
							Version:        "1.2.0",
							ExpirationDate: expirationDate,
						},
					}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[].expirationDate", fldPath)),
					}))
				})

				It("should forbid duplicated kubernetes versions", func() {
					azureCloudProfile.Spec.Azure.Constraints.Kubernetes.OfferedVersions = duplicatedKubernetes

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[%d].version", fldPath, len(duplicatedKubernetes)-1)),
						}))))
				})
			})

			Context("machine image validation", func() {
				It("should forbid an empty list of machine images", func() {
					azureCloudProfile.Spec.Azure.Constraints.MachineImages = []garden.MachineImage{}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages", fldPath)),
					}))
				})

				It("should forbid duplicate names in list of machine images", func() {
					azureCloudProfile.Spec.Azure.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
								{
									Version: "3.4.6"},
							},
						},
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
								{
									Version: "3.4.5",
								},
							},
						},
					}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[1]", fldPath)),
					}))
				})

				It("should forbid machine images with empty image names", func() {
					azureCloudProfile.Spec.Azure.Constraints.MachineImages = []garden.MachineImage{
						{},
					}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(HaveLen(2))

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[0].name", fldPath)),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[0].versions", fldPath)),
					}))))
				})
			})

			Context("machine types validation", func() {
				It("should enforce that at least one machine type has been defined", func() {
					azureCloudProfile.Spec.Azure.Constraints.MachineTypes = []garden.MachineType{}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes", fldPath)),
					}))
				})

				It("should enforce uniqueness of machine type names", func() {
					azureCloudProfile.Spec.Azure.Constraints.MachineTypes = []garden.MachineType{
						azureCloudProfile.Spec.Azure.Constraints.MachineTypes[0],
						azureCloudProfile.Spec.Azure.Constraints.MachineTypes[0],
					}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[1].name", fldPath)),
					}))))
				})

				It("should forbid machine types with unsupported property values", func() {
					azureCloudProfile.Spec.Azure.Constraints.MachineTypes = invalidMachineTypes

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(HaveLen(4))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].name", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].cpu", fldPath)),
					}))
					Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].gpu", fldPath)),
					}))
				})
			})

			Context("volume types validation", func() {
				It("should enforce uniqueness of volume type names", func() {
					azureCloudProfile.Spec.Azure.Constraints.VolumeTypes = []garden.VolumeType{
						azureCloudProfile.Spec.Azure.Constraints.VolumeTypes[0],
						azureCloudProfile.Spec.Azure.Constraints.VolumeTypes[0],
					}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[1].name", fldPath)),
					}))))
				})

				It("should forbid volume types with unsupported property values", func() {
					azureCloudProfile.Spec.Azure.Constraints.VolumeTypes = invalidVolumeTypes

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[0].name", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[0].class", fldPath)),
					}))
				})
			})

			Context("fault domain count validation", func() {
				It("should enforce that at least one fault domain count has been defined", func() {
					azureCloudProfile.Spec.Azure.CountFaultDomains = []garden.AzureDomainCount{}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.countFaultDomains", fldPath)),
					}))
				})

				It("should forbid fault domain count with unsupported format", func() {
					azureCloudProfile.Spec.Azure.CountFaultDomains = []garden.AzureDomainCount{
						{
							Region: "",
							Count:  -1,
						},
					}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.countFaultDomains[0].region", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.countFaultDomains[0].count", fldPath)),
					}))
				})
			})

			Context("update domain count validation", func() {
				It("should enforce that at least one update domain count has been defined", func() {
					azureCloudProfile.Spec.Azure.CountUpdateDomains = []garden.AzureDomainCount{}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.countUpdateDomains", fldPath)),
					}))
				})

				It("should forbid update domain count with unsupported format", func() {
					azureCloudProfile.Spec.Azure.CountUpdateDomains = []garden.AzureDomainCount{
						{
							Region: "",
							Count:  -1,
						},
					}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.countUpdateDomains[0].region", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.countUpdateDomains[0].count", fldPath)),
					}))
				})
			})
		})

		Context("tests for GCP cloud profiles", func() {
			var (
				fldPath         = "gcp"
				gcpCloudProfile *garden.CloudProfile
			)

			BeforeEach(func() {
				gcpCloudProfile = &garden.CloudProfile{
					ObjectMeta: metadata,
					Spec: garden.CloudProfileSpec{
						GCP: &garden.GCPProfile{
							Constraints: garden.GCPConstraints{
								Kubernetes: kubernetesVersionConstraint,
								MachineImages: []garden.MachineImage{
									{
										Name:     "some-machineimage",
										Versions: []garden.MachineImageVersion{{Version: "1.6.4"}},
									},
								},
								MachineTypes: machineTypesConstraint,
								VolumeTypes:  volumeTypesConstraint,
								Zones:        zonesConstraint,
							},
						},
						Type: "gcp",
						Kubernetes: garden.KubernetesSettings{
							Versions: []garden.ExpirableVersion{{Version: "1.11.4"}},
						},
						MachineImages: []garden.CloudProfileMachineImage{
							{
								Name: "some-machineimage",
								Versions: []garden.ExpirableVersion{
									{Version: "1.2.3"},
								},
							},
						},
						MachineTypes: machineTypesConstraint,
					},
				}
			})

			Context("kubernetes version constraints", func() {
				It("should enforce that at least one version has been defined", func() {
					gcpCloudProfile.Spec.GCP.Constraints.Kubernetes.OfferedVersions = []garden.KubernetesVersion{}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions", fldPath)),
					}))
				})

				It("should forbid versions of a not allowed pattern", func() {
					gcpCloudProfile.Spec.GCP.Constraints.Kubernetes.OfferedVersions = invalidKubernetes

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[0]", fldPath)),
					}))
				})

				It("should forbid expiration date on latest kubernetes version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					gcpCloudProfile.Spec.GCP.Constraints.Kubernetes.OfferedVersions = []garden.KubernetesVersion{
						{
							Version: "1.1.0",
						},
						{
							Version:        "1.2.0",
							ExpirationDate: expirationDate,
						},
					}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[].expirationDate", fldPath)),
					}))
				})

				It("should forbid duplicated kubernetes versions", func() {
					gcpCloudProfile.Spec.GCP.Constraints.Kubernetes.OfferedVersions = duplicatedKubernetes

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[%d].version", fldPath, len(duplicatedKubernetes)-1)),
						}))))
				})
			})

			Context("machine image validation", func() {
				It("should forbid an empty list of machine images", func() {
					gcpCloudProfile.Spec.GCP.Constraints.MachineImages = []garden.MachineImage{}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages", fldPath)),
					}))
				})

				It("should forbid duplicate names in list of machine images", func() {
					gcpCloudProfile.Spec.GCP.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
								{
									Version: "3.4.6"},
							},
						},
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
								{
									Version: "3.4.5",
								},
							},
						},
					}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[1]", fldPath)),
					}))
				})

				It("should forbid machine images with no version", func() {
					gcpCloudProfile.Spec.GCP.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
						},
					}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[0].versions", fldPath)),
					}))))
				})

				It("should forbid nonSemVer machine image versions", func() {
					gcpCloudProfile.Spec.GCP.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
								{
									Version: "0.1.2"},
							},
						},
						{
							Name: "xy",
							Versions: []garden.MachineImageVersion{
								{
									Version: "a.b.c",
								},
							},
						},
					}
					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages", fldPath)),
					})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[1].versions[0].version", fldPath)),
						}))))
				})
				It("should forbid expiration date on latest machine image version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					gcpCloudProfile.Spec.GCP.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
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
							Versions: []garden.MachineImageVersion{
								{
									Version:        "0.1.1",
									ExpirationDate: expirationDate,
								},
							},
						},
					}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(fmt.Sprintf("spec.%s.constraints.machineImages.expirationDate", fldPath)),
						"Detail": ContainSubstring("some-machineimage"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(fmt.Sprintf("spec.%s.constraints.machineImages.expirationDate", fldPath)),
						"Detail": ContainSubstring("xy"),
					}))))
				})
			})

			Context("machine types validation", func() {
				It("should enforce that at least one machine type has been defined", func() {
					gcpCloudProfile.Spec.GCP.Constraints.MachineTypes = []garden.MachineType{}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes", fldPath)),
					}))
				})

				It("should enforce uniqueness of machine type names", func() {
					gcpCloudProfile.Spec.GCP.Constraints.MachineTypes = []garden.MachineType{
						gcpCloudProfile.Spec.GCP.Constraints.MachineTypes[0],
						gcpCloudProfile.Spec.GCP.Constraints.MachineTypes[0],
					}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[1].name", fldPath)),
					}))))
				})

				It("should forbid machine types with unsupported property values", func() {
					gcpCloudProfile.Spec.GCP.Constraints.MachineTypes = invalidMachineTypes

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(HaveLen(4))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].name", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].cpu", fldPath)),
					}))
					Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].gpu", fldPath)),
					}))
					Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].memory", fldPath)),
					}))
				})
			})

			Context("volume types validation", func() {
				It("should enforce uniqueness of volume type names", func() {
					gcpCloudProfile.Spec.GCP.Constraints.VolumeTypes = []garden.VolumeType{
						gcpCloudProfile.Spec.GCP.Constraints.VolumeTypes[0],
						gcpCloudProfile.Spec.GCP.Constraints.VolumeTypes[0],
					}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[1].name", fldPath)),
					}))))
				})

				It("should forbid volume types with unsupported property values", func() {
					gcpCloudProfile.Spec.GCP.Constraints.VolumeTypes = invalidVolumeTypes

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[0].name", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[0].class", fldPath)),
					}))
				})
			})

			Context("zones validation", func() {
				It("should enforce that at least one zone has been defined", func() {
					gcpCloudProfile.Spec.GCP.Constraints.Zones = []garden.Zone{}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones", fldPath)),
					}))
				})

				It("should forbid zones with unsupported name values", func() {
					gcpCloudProfile.Spec.GCP.Constraints.Zones = invalidZones

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].region", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].names[0]", fldPath)),
					}))
				})

				It("should forbid duplicated region names", func() {
					gcpCloudProfile.Spec.GCP.Constraints.Zones = duplicatedRegionsConstraint

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[%d].region", fldPath, len(duplicatedRegionsConstraint)-1)),
						}))))
				})

				It("should forbid duplicated zone names", func() {
					gcpCloudProfile.Spec.GCP.Constraints.Zones = duplicatedZonesConstraint

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].names[%d]", fldPath, len(duplicatedZonesConstraint[0].Names)-1)),
						}))))
				})
			})
		})

		Context("tests for OpenStack cloud profiles", func() {
			var (
				fldPath               = "openstack"
				openStackCloudProfile *garden.CloudProfile
			)

			BeforeEach(func() {
				openStackCloudProfile = &garden.CloudProfile{
					ObjectMeta: metadata,
					Spec: garden.CloudProfileSpec{
						OpenStack: &garden.OpenStackProfile{
							Constraints: garden.OpenStackConstraints{
								FloatingPools: []garden.OpenStackFloatingPool{
									{
										Name: "MY-POOL",
									},
								},
								Kubernetes: kubernetesVersionConstraint,
								LoadBalancerProviders: []garden.OpenStackLoadBalancerProvider{
									{
										Name: "haproxy",
									},
								},

								MachineImages: []garden.MachineImage{
									{
										Name:     "some-machineimage",
										Versions: []garden.MachineImageVersion{{Version: "1.6.4"}},
									},
								},
								MachineTypes: openStackMachineTypesConstraint,
								Zones:        zonesConstraint,
							},
							KeyStoneURL: "http://url-to-keystone/v3",
						},
						Type: "openstack",
						Kubernetes: garden.KubernetesSettings{
							Versions: []garden.ExpirableVersion{{Version: "1.11.4"}},
						},
						MachineImages: []garden.CloudProfileMachineImage{
							{
								Name: "some-machineimage",
								Versions: []garden.ExpirableVersion{
									{Version: "1.2.3"},
								},
							},
						},
						MachineTypes: machineTypesConstraint,
					},
				}
			})

			Context("floating pools constraints", func() {
				It("should enforce that at least one pool has been defined", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.FloatingPools = []garden.OpenStackFloatingPool{}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.floatingPools", fldPath)),
					}))
				})

				It("should forbid unsupported providers", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.FloatingPools = []garden.OpenStackFloatingPool{
						{
							Name: "",
						},
					}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.floatingPools[0].name", fldPath)),
					}))
				})
			})

			Context("kubernetes version constraints", func() {
				It("should enforce that at least one version has been defined", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.Kubernetes.OfferedVersions = []garden.KubernetesVersion{}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions", fldPath)),
					}))
				})

				It("should forbid versions of a not allowed pattern", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.Kubernetes.OfferedVersions = invalidKubernetes

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[0]", fldPath)),
					}))
				})

				It("should forbid expiration date on latest kubernetes version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					openStackCloudProfile.Spec.OpenStack.Constraints.Kubernetes.OfferedVersions = []garden.KubernetesVersion{
						{
							Version: "1.1.0",
						},
						{
							Version:        "1.2.0",
							ExpirationDate: expirationDate,
						},
					}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[].expirationDate", fldPath)),
					}))
				})

				It("should forbid duplicated kubernetes versions", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.Kubernetes.OfferedVersions = duplicatedKubernetes

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[%d].version", fldPath, len(duplicatedKubernetes)-1)),
						}))))
				})
			})

			Context("load balancer provider constraints", func() {
				It("should enforce that at least one provider has been defined", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.LoadBalancerProviders = []garden.OpenStackLoadBalancerProvider{}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.loadBalancerProviders", fldPath)),
					}))
				})

				It("should forbid unsupported providers", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.LoadBalancerProviders = []garden.OpenStackLoadBalancerProvider{
						{
							Name: "",
						},
					}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.loadBalancerProviders[0].name", fldPath)),
					}))
				})
			})

			Context("machine image validation", func() {
				It("should forbid an empty list of machine images", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.MachineImages = []garden.MachineImage{}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages", fldPath)),
					}))
				})

				It("should forbid duplicate names in list of machine images", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
								{
									Version: "3.4.6"},
							},
						},
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
								{
									Version: "3.4.5",
								},
							},
						},
					}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[1]", fldPath)),
					}))
				})

				It("should forbid machine images with no version", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
						},
					}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[0].versions", fldPath)),
					}))))
				})

				It("should forbid nonSemVer machine image versions", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
								{
									Version: "0.1.2"},
							},
						},
						{
							Name: "xz",
							Versions: []garden.MachineImageVersion{
								{
									Version: "a.b.c",
								},
							},
						},
					}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages", fldPath)),
					})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[1].versions[0].version", fldPath)),
						}))))
				})
				It("should forbid expiration date on latest machine image version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					openStackCloudProfile.Spec.OpenStack.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
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
							Versions: []garden.MachineImageVersion{
								{
									Version:        "0.1.1",
									ExpirationDate: expirationDate,
								},
							},
						},
					}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(fmt.Sprintf("spec.%s.constraints.machineImages.expirationDate", fldPath)),
						"Detail": ContainSubstring("some-machineimage"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(fmt.Sprintf("spec.%s.constraints.machineImages.expirationDate", fldPath)),
						"Detail": ContainSubstring("xy"),
					}))))
				})
			})

			Context("machine types validation", func() {
				It("should enforce that at least one machine type has been defined", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.MachineTypes = []garden.OpenStackMachineType{}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes", fldPath)),
					}))
				})

				It("should enforce uniqueness of machine type names", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.MachineTypes = []garden.OpenStackMachineType{
						openStackCloudProfile.Spec.OpenStack.Constraints.MachineTypes[0],
						openStackCloudProfile.Spec.OpenStack.Constraints.MachineTypes[0],
					}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[1].name", fldPath)),
					}))))
				})

				It("should forbid machine types with unsupported property values", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.MachineTypes = invalidOpenStackMachineTypes

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(6))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].volumeType", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].volumeSize", fldPath)),
					}))
					Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].name", fldPath)),
					}))
					Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].cpu", fldPath)),
					}))
					Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].gpu", fldPath)),
					}))
					Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].memory", fldPath)),
					}))
				})
			})

			Context("zones validation", func() {
				It("should enforce that at least one zone has been defined", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.Zones = []garden.Zone{}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones", fldPath)),
					}))
				})

				It("should forbid zones with unsupported name values", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.Zones = invalidZones

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].region", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].names[0]", fldPath)),
					}))
				})

				It("should forbid duplicated region names", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.Zones = duplicatedRegionsConstraint

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[%d].region", fldPath, len(duplicatedRegionsConstraint)-1)),
						}))))
				})

				It("should forbid duplicated zone names", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.Zones = duplicatedZonesConstraint

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].names[%d]", fldPath, len(duplicatedZonesConstraint[0].Names)-1)),
						}))))
				})
			})

			Context("keystone url validation", func() {
				It("should forbid keystone urls with unsupported format", func() {
					openStackCloudProfile.Spec.OpenStack.KeyStoneURL = ""

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.keystoneURL", fldPath)),
					}))
				})
			})

			Context("dhcp domain validation", func() {
				It("should forbid not specifying a value when the key is present", func() {
					openStackCloudProfile.Spec.OpenStack.DHCPDomain = makeStringPointer("")

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.dhcpDomain", fldPath)),
					}))
				})
			})

			Context("requestTimeout validation", func() {
				It("should reject invalid durations", func() {
					openStackCloudProfile.Spec.OpenStack.RequestTimeout = makeStringPointer("1GiB")

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.requestTimeout", fldPath)),
					}))))
				})
			})
		})

		Context("tests for Alicloud cloud profiles", func() {
			var (
				fldPath         = "alicloud"
				alicloudProfile *garden.CloudProfile
			)

			BeforeEach(func() {
				alicloudProfile = &garden.CloudProfile{
					ObjectMeta: metadata,
					Spec: garden.CloudProfileSpec{
						Alicloud: &garden.AlicloudProfile{
							Constraints: garden.AlicloudConstraints{
								Kubernetes: kubernetesVersionConstraint,
								MachineImages: []garden.MachineImage{
									{
										Name:     "some-machineimage",
										Versions: []garden.MachineImageVersion{{Version: "1.0.0"}},
									},
								},
								MachineTypes: []garden.AlicloudMachineType{
									{
										MachineType: garden.MachineType{
											Name:   "ecs.sn2ne.large",
											CPU:    resource.MustParse("2"),
											GPU:    resource.MustParse("0"),
											Memory: resource.MustParse("8Gi"),
										},
										Zones: []string{
											"my-region-a",
										},
									},
								},
								VolumeTypes: []garden.AlicloudVolumeType{
									{
										VolumeType: garden.VolumeType{
											Name:  "cloud_efficiency",
											Class: "standard",
										},
										Zones: []string{
											"my-region-a",
										},
									},
								},
								Zones: zonesConstraint,
							},
						},
						Type: "alicloud",
						Kubernetes: garden.KubernetesSettings{
							Versions: []garden.ExpirableVersion{{Version: "1.11.4"}},
						},
						MachineImages: []garden.CloudProfileMachineImage{
							{
								Name: "some-machineimage",
								Versions: []garden.ExpirableVersion{
									{Version: "1.2.3"},
								},
							},
						},
						MachineTypes: machineTypesConstraint,
					},
				}
			})

			It("should not return any errors", func() {
				errorList := ValidateCloudProfile(alicloudProfile)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid ca bundles with unsupported format", func() {
				alicloudProfile.Spec.CABundle = makeStringPointer("unsupported")

				errorList := ValidateCloudProfile(alicloudProfile)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.caBundle"),
				}))
			})

			Context("kubernetes version constraints", func() {
				It("should enforce that at least one version has been defined", func() {
					alicloudProfile.Spec.Alicloud.Constraints.Kubernetes.OfferedVersions = []garden.KubernetesVersion{}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions", fldPath)),
					}))
				})

				It("should forbid versions of a not allowed pattern", func() {
					alicloudProfile.Spec.Alicloud.Constraints.Kubernetes.OfferedVersions = invalidKubernetes

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[0]", fldPath)),
					}))
				})

				It("should forbid expiration date on latest kubernetes version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					alicloudProfile.Spec.Alicloud.Constraints.Kubernetes.OfferedVersions = []garden.KubernetesVersion{
						{
							Version: "1.1.0",
						},
						{
							Version:        "1.2.0",
							ExpirationDate: expirationDate,
						},
					}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[].expirationDate", fldPath)),
					}))
				})

				It("should forbid duplicated kubernetes versions", func() {
					alicloudProfile.Spec.Alicloud.Constraints.Kubernetes.OfferedVersions = duplicatedKubernetes

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[%d].version", fldPath, len(duplicatedKubernetes)-1)),
						}))))
				})
			})

			Context("machine image validation", func() {
				It("should forbid an empty list of machine images", func() {
					alicloudProfile.Spec.Alicloud.Constraints.MachineImages = []garden.MachineImage{}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages", fldPath)),
					}))
				})

				It("should forbid empty machine image versions slice", func() {
					alicloudProfile.Spec.Alicloud.Constraints.MachineImages[0].Versions = []garden.MachineImageVersion{}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[0].versions", fldPath)),
						})),
					))
				})
				It("should forbid nonSemVer machine image versions", func() {
					alicloudProfile.Spec.Alicloud.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
								{
									Version: "0.1.2"},
							},
						},
						{
							Name: "xy",
							Versions: []garden.MachineImageVersion{
								{
									Version: "a.b.c",
								},
							},
						},
					}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages", fldPath)),
					})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[1].versions[0].version", fldPath)),
						}))))
				})
				It("should forbid expiration date on latest machine image version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					alicloudProfile.Spec.Alicloud.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
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
							Versions: []garden.MachineImageVersion{
								{
									Version:        "0.1.1",
									ExpirationDate: expirationDate,
								},
							},
						},
					}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(fmt.Sprintf("spec.%s.constraints.machineImages.expirationDate", fldPath)),
						"Detail": ContainSubstring("some-machineimage"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(fmt.Sprintf("spec.%s.constraints.machineImages.expirationDate", fldPath)),
						"Detail": ContainSubstring("xy"),
					}))))
				})
			})

			Context("machine types validation", func() {
				It("should enforce that at least one machine type has been defined", func() {
					alicloudProfile.Spec.Alicloud.Constraints.MachineTypes = []garden.AlicloudMachineType{}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes", fldPath)),
					}))
				})

				It("should enforce uniqueness of machine type names", func() {
					alicloudProfile.Spec.Alicloud.Constraints.MachineTypes = []garden.AlicloudMachineType{
						alicloudProfile.Spec.Alicloud.Constraints.MachineTypes[0],
						alicloudProfile.Spec.Alicloud.Constraints.MachineTypes[0],
					}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[1].name", fldPath)),
					}))))
				})

				It("should forbid machine types with unsupported property values", func() {
					alicloudProfile.Spec.Alicloud.Constraints.MachineTypes = []garden.AlicloudMachineType{
						{
							MachineType: garden.MachineType{
								Name:   "",
								CPU:    resource.MustParse("-2"),
								GPU:    resource.MustParse("-2"),
								Memory: resource.MustParse("-8Gi"),
							},
						},
					}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(4))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].name", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].cpu", fldPath)),
					}))
					Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].gpu", fldPath)),
					}))
					Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].memory", fldPath)),
					}))
				})

				It("should enforce zone name in general zones defined in constraints", func() {
					alicloudProfile.Spec.Alicloud.Constraints.MachineTypes = []garden.AlicloudMachineType{
						{
							MachineType: garden.MachineType{
								Name:   "ecs.sn2ne.large",
								CPU:    resource.MustParse("2"),
								GPU:    resource.MustParse("0"),
								Memory: resource.MustParse("8Gi"),
							},
							Zones: []string{
								"cn-beijing-",
							},
						},
					}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].zones[0]", fldPath)),
					}))
				})
			})

			Context("volume types validation", func() {
				It("should enforce uniqueness of volume type names", func() {
					alicloudProfile.Spec.Alicloud.Constraints.VolumeTypes = []garden.AlicloudVolumeType{
						alicloudProfile.Spec.Alicloud.Constraints.VolumeTypes[0],
						alicloudProfile.Spec.Alicloud.Constraints.VolumeTypes[0],
					}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[1].name", fldPath)),
					}))))
				})

				It("should forbid volume types with unsupported property values", func() {
					alicloudProfile.Spec.Alicloud.Constraints.VolumeTypes = []garden.AlicloudVolumeType{
						{
							VolumeType: garden.VolumeType{
								Name:  "",
								Class: "",
							},
						},
					}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[0].name", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[0].class", fldPath)),
					}))
				})

				It("should enforce zone name in general zones defined in constraints", func() {
					alicloudProfile.Spec.Alicloud.Constraints.VolumeTypes = []garden.AlicloudVolumeType{
						{
							VolumeType: garden.VolumeType{
								Name:  "cloud_efficiency",
								Class: "standard",
							},
							Zones: []string{
								"cn-beijing-",
							},
						},
					}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[0].zones[0]", fldPath)),
					}))
				})
			})

			Context("zone validation", func() {
				It("should forbid empty zones", func() {
					alicloudProfile.Spec.Alicloud.Constraints.Zones = []garden.Zone{}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(3))
					Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones", fldPath)),
					}))
				})

				It("should forbid zones with unsupported name values", func() {
					alicloudProfile.Spec.Alicloud.Constraints.Zones = invalidZones

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(4))
					Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].region", fldPath)),
					}))
					Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].names[0]", fldPath)),
					}))
				})

				It("should forbid duplicated region names", func() {
					alicloudProfile.Spec.Alicloud.Constraints.Zones = duplicatedRegionsConstraint

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[%d].region", fldPath, len(duplicatedRegionsConstraint)-1)),
						}))))
				})

				It("should forbid duplicated zone names", func() {
					alicloudProfile.Spec.Alicloud.Constraints.Zones = duplicatedZonesConstraint

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].names[%d]", fldPath, len(duplicatedZonesConstraint[0].Names)-1)),
						}))))
				})
			})
		})

		// BEGIN PACKET
		Context("tests for Packet cloud profiles", func() {
			var (
				fldPath       = "packet"
				packetProfile *garden.CloudProfile
			)

			BeforeEach(func() {
				packetProfile = &garden.CloudProfile{
					ObjectMeta: metadata,
					Spec: garden.CloudProfileSpec{
						Packet: &garden.PacketProfile{
							Constraints: garden.PacketConstraints{
								Kubernetes: kubernetesVersionConstraint,
								MachineImages: []garden.MachineImage{
									{
										Name:     "Container Linux - Stable",
										Versions: []garden.MachineImageVersion{{Version: "2135.5.0"}},
									},
								},
								MachineTypes: machineTypesConstraint,
								VolumeTypes:  volumeTypesConstraint,
								Zones:        zonesConstraint,
							},
						},
						Type: "packet",
						Kubernetes: garden.KubernetesSettings{
							Versions: []garden.ExpirableVersion{{Version: "1.11.4"}},
						},
						MachineImages: []garden.CloudProfileMachineImage{
							{
								Name: "some-machineimage",
								Versions: []garden.ExpirableVersion{
									{Version: "1.2.3"},
								},
							},
						},
						MachineTypes: machineTypesConstraint,
					},
				}
			})

			It("should not return any errors", func() {
				errorList := ValidateCloudProfile(packetProfile)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid ca bundles with unsupported format", func() {
				packetProfile.Spec.CABundle = makeStringPointer("unsupported")

				errorList := ValidateCloudProfile(packetProfile)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.caBundle"),
				}))))

			})

			Context("kubernetes version constraints", func() {
				It("should enforce that at least one version has been defined", func() {
					packetProfile.Spec.Packet.Constraints.Kubernetes.OfferedVersions = []garden.KubernetesVersion{}

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions", fldPath)),
					}))))
				})

				It("should forbid versions of a not allowed pattern", func() {
					packetProfile.Spec.Packet.Constraints.Kubernetes.OfferedVersions = invalidKubernetes

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[0]", fldPath)),
					}))))
				})

				It("should forbid expiration date on latest kubernetes version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					packetProfile.Spec.Packet.Constraints.Kubernetes.OfferedVersions = []garden.KubernetesVersion{
						{
							Version: "1.1.0",
						},
						{
							Version:        "1.2.0",
							ExpirationDate: expirationDate,
						},
					}

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[].expirationDate", fldPath)),
					}))
				})

				It("should forbid duplicated kubernetes versions", func() {
					packetProfile.Spec.Packet.Constraints.Kubernetes.OfferedVersions = duplicatedKubernetes

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.offeredVersions[%d].version", fldPath, len(duplicatedKubernetes)-1)),
						}))))
				})
			})

			Context("machine image validation", func() {
				It("should forbid an empty list of machine images", func() {
					packetProfile.Spec.Packet.Constraints.MachineImages = []garden.MachineImage{}

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages", fldPath)),
					}))))
				})

				It("should forbid empty machine image versions slice", func() {
					packetProfile.Spec.Packet.Constraints.MachineImages[0].Versions = []garden.MachineImageVersion{}

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(errorList).To(HaveLen(1))
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[0].versions", fldPath)),
						})),
					))
				})

				It("should forbid nonSemVer machine image versions", func() {
					packetProfile.Spec.Packet.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
								{
									Version: "0.1.2"},
							},
						},
						{
							Name: "xy",
							Versions: []garden.MachineImageVersion{
								{
									Version: "a.b.c",
								},
							},
						},
					}

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages", fldPath)),
					})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineImages[1].versions[0].version", fldPath)),
						}))))
				})
				It("should forbid expiration date on latest machine image version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					packetProfile.Spec.Packet.Constraints.MachineImages = []garden.MachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.MachineImageVersion{
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
							Versions: []garden.MachineImageVersion{
								{
									Version:        "0.1.1",
									ExpirationDate: expirationDate,
								},
							},
						},
					}

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(fmt.Sprintf("spec.%s.constraints.machineImages.expirationDate", fldPath)),
						"Detail": ContainSubstring("some-machineimage"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(fmt.Sprintf("spec.%s.constraints.machineImages.expirationDate", fldPath)),
						"Detail": ContainSubstring("xy"),
					}))))
				})
			})

			Context("machine types validation", func() {
				It("should enforce that at least one machine type has been defined", func() {
					packetProfile.Spec.Packet.Constraints.MachineTypes = []garden.MachineType{}

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes", fldPath)),
					}))))
				})

				It("should enforce uniqueness of machine type names", func() {
					packetProfile.Spec.Packet.Constraints.MachineTypes = []garden.MachineType{
						packetProfile.Spec.Packet.Constraints.MachineTypes[0],
						packetProfile.Spec.Packet.Constraints.MachineTypes[0],
					}

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[1].name", fldPath)),
					}))))
				})

				It("should forbid machine types with unsupported property values", func() {
					packetProfile.Spec.Packet.Constraints.MachineTypes = invalidMachineTypes

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].name", fldPath)),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].cpu", fldPath)),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].gpu", fldPath)),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].memory", fldPath)),
					}))))
				})
			})

			Context("volume types validation", func() {
				It("should enforce uniqueness of volume type names", func() {
					packetProfile.Spec.Packet.Constraints.VolumeTypes = []garden.VolumeType{
						packetProfile.Spec.Packet.Constraints.VolumeTypes[0],
						packetProfile.Spec.Packet.Constraints.VolumeTypes[0],
					}

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[1].name", fldPath)),
					}))))
				})

				It("should forbid volume types with unsupported property values", func() {
					packetProfile.Spec.Packet.Constraints.VolumeTypes = invalidVolumeTypes

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[0].name", fldPath)),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes[0].class", fldPath)),
					}))))
				})

			})

			Context("zone validation", func() {
				It("should forbid empty zones", func() {
					packetProfile.Spec.Packet.Constraints.Zones = []garden.Zone{}

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones", fldPath)),
					}))))
				})

				It("should forbid zones with unsupported name values", func() {
					packetProfile.Spec.Packet.Constraints.Zones = invalidZones

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(HaveLen(2))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].region", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].names[0]", fldPath)),
					}))
				})

				It("should forbid duplicated region names", func() {
					packetProfile.Spec.Packet.Constraints.Zones = duplicatedRegionsConstraint

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[%d].region", fldPath, len(duplicatedRegionsConstraint)-1)),
						}))))
				})

				It("should forbid duplicated zone names", func() {
					packetProfile.Spec.Packet.Constraints.Zones = duplicatedZonesConstraint

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones[0].names[%d]", fldPath, len(duplicatedZonesConstraint[0].Names)-1)),
						}))))
				})
			})
		})
		// END PACKET

		Context("tests for unknown cloud profiles", func() {
			var (
				regionName = "region1"
				zoneName   = "zone1"

				unknownCloudProfile *garden.CloudProfile

				duplicatedKubernetes = garden.KubernetesSettings{
					Versions: []garden.ExpirableVersion{{Version: "1.11.4"}, {Version: "1.11.4"}},
				}
				duplicatedRegions = []garden.Region{
					{
						Name: regionName,
						Zones: []garden.AvailabilityZone{
							{Name: zoneName},
						},
					},
					{
						Name: regionName,
						Zones: []garden.AvailabilityZone{
							{Name: zoneName},
						},
					},
				}
				duplicatedZones = []garden.Region{
					{
						Name: regionName,
						Zones: []garden.AvailabilityZone{
							{Name: zoneName},
							{Name: zoneName},
						},
					},
				}
			)

			BeforeEach(func() {
				unknownCloudProfile = &garden.CloudProfile{
					ObjectMeta: metadata,
					Spec: garden.CloudProfileSpec{
						Type: "unknown",
						SeedSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
						Kubernetes: garden.KubernetesSettings{
							Versions: []garden.ExpirableVersion{{Version: "1.11.4"}},
						},
						MachineImages: []garden.CloudProfileMachineImage{
							{
								Name: "some-machineimage",
								Versions: []garden.ExpirableVersion{
									{Version: "1.2.3"},
								},
							},
						},
						Regions: []garden.Region{
							{
								Name: regionName,
								Zones: []garden.AvailabilityZone{
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
					unknownCloudProfile.Spec.Kubernetes.Versions = []garden.ExpirableVersion{}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.kubernetes.versions"),
					}))))
				})

				It("should forbid versions of a not allowed pattern", func() {
					unknownCloudProfile.Spec.Kubernetes.Versions = []garden.ExpirableVersion{{Version: "1.11"}}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.versions[0]"),
					}))))
				})

				It("should forbid expiration date on latest kubernetes version", func() {
					expirationDate := &metav1.Time{Time: time.Now().AddDate(0, 0, 1)}
					unknownCloudProfile.Spec.Kubernetes.Versions = []garden.ExpirableVersion{
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
					unknownCloudProfile.Spec.MachineImages = []garden.CloudProfileMachineImage{}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineImages"),
					}))))
				})

				It("should forbid duplicate names in list of machine images", func() {
					unknownCloudProfile.Spec.MachineImages = []garden.CloudProfileMachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.ExpirableVersion{
								{Version: "3.4.6"},
							},
						},
						{
							Name: "some-machineimage",
							Versions: []garden.ExpirableVersion{
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
					unknownCloudProfile.Spec.MachineImages = []garden.CloudProfileMachineImage{
						{Name: "some-machineimage"},
					}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineImages[0].versions"),
					}))))
				})

				It("should forbid nonSemVer machine image versions", func() {
					unknownCloudProfile.Spec.MachineImages = []garden.CloudProfileMachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.ExpirableVersion{
								{
									Version: "0.1.2"},
							},
						},
						{
							Name: "xy",
							Versions: []garden.ExpirableVersion{
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
					unknownCloudProfile.Spec.MachineImages = []garden.CloudProfileMachineImage{
						{
							Name: "some-machineimage",
							Versions: []garden.ExpirableVersion{
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
							Versions: []garden.ExpirableVersion{
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
					unknownCloudProfile.Spec.MachineTypes = []garden.MachineType{}

					errorList := ValidateCloudProfile(unknownCloudProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.machineTypes"),
					}))))
				})

				It("should enforce uniqueness of machine type names", func() {
					unknownCloudProfile.Spec.MachineTypes = []garden.MachineType{
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
					unknownCloudProfile.Spec.Regions = []garden.Region{
						{
							Name:  "",
							Zones: []garden.AvailabilityZone{{Name: ""}},
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
					unknownCloudProfile.Spec.VolumeTypes = []garden.VolumeType{
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
