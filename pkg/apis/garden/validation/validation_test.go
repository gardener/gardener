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
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/apis/garden"
	. "github.com/gardener/gardener/pkg/apis/garden/validation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/validation/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
)

var _ = Describe("validation", func() {
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

			invalidKubernetes  = []garden.KubernetesVersion{{Version: "1.11"}}
			invalidMachineType = garden.MachineType{
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
		)

		It("should forbid empty CloudProfile resources", func() {
			cloudProfile := &garden.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       garden.CloudProfileSpec{},
			}

			errorList := ValidateCloudProfile(cloudProfile)

			Expect(errorList).To(HaveLen(2))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.aws/azure/gcp/alicloud/openstack/packet"),
			}))
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
				It("should enforce that at least one volume type has been defined", func() {
					awsCloudProfile.Spec.AWS.Constraints.VolumeTypes = []garden.VolumeType{}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes", fldPath)),
					}))
				})

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
				It("should enforce that at least one volume type has been defined", func() {
					azureCloudProfile.Spec.Azure.Constraints.VolumeTypes = []garden.VolumeType{}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes", fldPath)),
					}))
				})

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
				It("should enforce that at least one volume type has been defined", func() {
					gcpCloudProfile.Spec.GCP.Constraints.VolumeTypes = []garden.VolumeType{}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes", fldPath)),
					}))
				})

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
			})

			Context("keystone url validation", func() {
				It("should forbid keystone urls with unsupported format", func() {
					openStackCloudProfile.Spec.OpenStack.KeyStoneURL = ""

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.keyStoneURL", fldPath)),
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
				It("should enforce that at least one volume type has been defined", func() {
					alicloudProfile.Spec.Alicloud.Constraints.VolumeTypes = []garden.AlicloudVolumeType{}

					errorList := ValidateCloudProfile(alicloudProfile)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes", fldPath)),
					}))
				})

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
				It("should enforce that at least one volume type has been defined", func() {
					packetProfile.Spec.Packet.Constraints.VolumeTypes = []garden.VolumeType{}

					errorList := ValidateCloudProfile(packetProfile)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes", fldPath)),
					}))))
				})

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
			})
		})
		// END PACKET
	})

	Describe("#ValidateProject, #ValidateProjectUpdate", func() {
		var project *garden.Project

		BeforeEach(func() {
			project = &garden.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "project-1",
				},
				Spec: garden.ProjectSpec{
					CreatedBy: &rbacv1.Subject{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     rbacv1.UserKind,
						Name:     "john.doe@example.com",
					},
					Owner: &rbacv1.Subject{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     rbacv1.UserKind,
						Name:     "john.doe@example.com",
					},
					ProjectMembers: []garden.ProjectMember{
						{
							Subject: rbacv1.Subject{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     rbacv1.UserKind,
								Name:     "alice.doe@example.com",
							},
							Role: garden.ProjectMemberAdmin,
						},
						{
							Subject: rbacv1.Subject{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     rbacv1.UserKind,
								Name:     "bob.doe@example.com",
							},
							Role: garden.ProjectMemberViewer,
						},
					},
				},
			}
		})

		It("should not return any errors", func() {
			errorList := ValidateProject(project)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid Project resources with empty metadata", func() {
			project.ObjectMeta = metav1.ObjectMeta{}

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			}))))
		})

		It("should forbid Projects having too long names", func() {
			project.ObjectMeta.Name = "project-name-too-long"

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeTooLong),
				"Field": Equal("metadata.name"),
			}))))
		})

		It("should forbid Projects having two consecutive hyphens", func() {
			project.ObjectMeta.Name = "in--valid"

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("metadata.name"),
			}))))
		})

		It("should forbid Project specification with empty or invalid keys for description/purpose", func() {
			project.Spec.Description = makeStringPointer("")
			project.Spec.Purpose = makeStringPointer("")

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.description"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.purpose"),
			}))))
		})

		DescribeTable("owner validation",
			func(apiGroup, kind, name, namespace string, expectType field.ErrorType, field string) {
				subject := rbacv1.Subject{
					APIGroup:  apiGroup,
					Kind:      kind,
					Name:      name,
					Namespace: namespace,
				}

				project.Spec.Owner = &subject
				project.Spec.CreatedBy = &subject
				project.Spec.ProjectMembers = []garden.ProjectMember{
					{
						Subject: subject,
						Role:    garden.ProjectMemberAdmin,
					},
				}

				errList := ValidateProject(project)

				Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(expectType),
					"Field": Equal(fmt.Sprintf("spec.owner.%s", field)),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(expectType),
					"Field": Equal(fmt.Sprintf("spec.createdBy.%s", field)),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(expectType),
					"Field": Equal(fmt.Sprintf("spec.members[0].%s", field)),
				}))))
			},

			// general
			Entry("empty name", "rbac.authorization.k8s.io", rbacv1.UserKind, "", "", field.ErrorTypeRequired, "name"),
			Entry("unknown kind", "rbac.authorization.k8s.io", "unknown", "foo", "", field.ErrorTypeNotSupported, "kind"),

			// serviceaccounts
			Entry("invalid api group name", "apps/v1beta1", rbacv1.ServiceAccountKind, "foo", "default", field.ErrorTypeNotSupported, "apiGroup"),
			Entry("invalid name", "", rbacv1.ServiceAccountKind, "foo-", "default", field.ErrorTypeInvalid, "name"),
			Entry("no namespace", "", rbacv1.ServiceAccountKind, "foo", "", field.ErrorTypeRequired, "namespace"),

			// users
			Entry("invalid api group name", "rbac.authorization.invalid", rbacv1.UserKind, "john.doe@example.com", "", field.ErrorTypeNotSupported, "apiGroup"),

			// groups
			Entry("invalid api group name", "rbac.authorization.invalid", rbacv1.GroupKind, "groupname", "", field.ErrorTypeNotSupported, "apiGroup"),
		)

		DescribeTable("namespace immutability",
			func(old, new *string, matcher gomegatypes.GomegaMatcher) {
				project.Spec.Namespace = old
				newProject := prepareProjectForUpdate(project)
				newProject.Spec.Namespace = new

				errList := ValidateProjectUpdate(newProject, project)

				Expect(errList).To(matcher)
			},

			Entry("namespace change w/  preset namespace", makeStringPointer("garden-dev"), makeStringPointer("garden-core"), ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.namespace"),
			})))),
			Entry("namespace change w/o preset namespace", nil, makeStringPointer("garden-core"), BeEmpty()),
			Entry("no change (both unset)", nil, nil, BeEmpty()),
			Entry("no change (same value)", makeStringPointer("garden-dev"), makeStringPointer("garden-dev"), BeEmpty()),
		)

		It("should forbid Project updates trying to change the createdBy field", func() {
			newProject := prepareProjectForUpdate(project)
			newProject.Spec.CreatedBy.Name = "some-other-user"

			errorList := ValidateProjectUpdate(newProject, project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.createdBy"),
			}))))
		})

		It("should forbid Project updates trying to reset the owner field", func() {
			newProject := prepareProjectForUpdate(project)
			newProject.Spec.Owner = nil

			errorList := ValidateProjectUpdate(newProject, project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.owner"),
			}))))
		})
	})

	Describe("#ValidateSeed, #ValidateSeedUpdate", func() {
		var (
			seed   *garden.Seed
			backup *garden.SeedBackup
		)

		BeforeEach(func() {
			region := "some-region"
			pods := garden.CIDR("10.240.0.0/16")
			services := garden.CIDR("10.241.0.0/16")
			seed = &garden.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "seed-1",
					Annotations: map[string]string{
						common.AnnotatePersistentVolumeMinimumSize: "10Gi",
					},
				},
				Spec: garden.SeedSpec{
					Cloud: garden.SeedCloud{
						Profile: "aws",
						Region:  "eu-west-1",
					},
					Provider: garden.SeedProvider{
						Type:   "aws",
						Region: "eu-west-1",
					},
					IngressDomain: "ingress.my-seed-1.example.com",
					SecretRef: corev1.SecretReference{
						Name:      "seed-aws",
						Namespace: "garden",
					},
					Taints: []garden.SeedTaint{
						{Key: garden.SeedTaintProtected},
					},
					Networks: garden.SeedNetworks{
						Nodes:    garden.CIDR("10.250.0.0/16"),
						Pods:     garden.CIDR("100.96.0.0/11"),
						Services: garden.CIDR("100.64.0.0/13"),
						ShootDefaults: &garden.ShootNetworks{
							Pods:     &pods,
							Services: &services,
						},
					},
					Backup: &garden.SeedBackup{
						Provider: garden.CloudProviderAWS,
						Region:   &region,
						SecretRef: corev1.SecretReference{
							Name:      "backup-aws",
							Namespace: "garden",
						},
					},
				},
			}
		})

		It("should not return any errors", func() {
			errorList := ValidateSeed(seed)

			Expect(errorList).To(HaveLen(0))
		})

		It("should forbid Seed resources with empty metadata", func() {
			seed.ObjectMeta = metav1.ObjectMeta{}

			errorList := ValidateSeed(seed)

			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			}))
		})

		It("should forbid invalid annotations", func() {
			seed.ObjectMeta.Annotations = map[string]string{
				common.AnnotatePersistentVolumeMinimumSize: "10Gix",
			}
			errorList := ValidateSeed(seed)
			Expect(errorList).To(HaveLen(1))
		})

		It("should forbid Seed specification with empty or invalid keys", func() {
			invalidCIDR := garden.CIDR("invalid-cidr")
			seed.Spec.Cloud = garden.SeedCloud{}
			seed.Spec.Provider = garden.SeedProvider{}
			seed.Spec.IngressDomain = "invalid_dns1123-subdomain"
			seed.Spec.SecretRef = corev1.SecretReference{}
			seed.Spec.Networks = garden.SeedNetworks{
				Nodes:    invalidCIDR,
				Pods:     garden.CIDR("300.300.300.300/300"),
				Services: invalidCIDR,
				ShootDefaults: &garden.ShootNetworks{
					Pods:     &invalidCIDR,
					Services: &invalidCIDR,
				},
			}
			seed.Spec.Taints = []garden.SeedTaint{
				{Key: garden.SeedTaintProtected},
				{Key: garden.SeedTaintProtected},
				{Key: ""},
			}
			seed.Spec.Backup.SecretRef = corev1.SecretReference{}
			seed.Spec.Backup.Provider = ""
			minSize := resource.MustParse("-1")
			seed.Spec.Volume = &garden.SeedVolume{
				MinimumSize: &minSize,
				Providers: []garden.SeedVolumeProvider{
					{
						Purpose: "",
						Name:    "",
					},
					{
						Purpose: "duplicate",
						Name:    "value1",
					},
					{
						Purpose: "duplicate",
						Name:    "value2",
					},
				},
			}

			errorList := ValidateSeed(seed)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.backup.provider"),
					"Detail": Equal(`must provide a backup cloud provider name`),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.backup.secretRef.name"),
					"Detail": Equal(`must provide a name`),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.backup.secretRef.namespace"),
					"Detail": Equal(`must provide a namespace`),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.cloud.profile"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.cloud.region"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.provider.type"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.provider.region"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.ingressDomain"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.secretRef.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.secretRef.namespace"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.taints[1].key"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.taints[2].key"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.taints[2].key"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networks.nodes"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networks.pods"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networks.services"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networks.shootDefaults.pods"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networks.shootDefaults.services"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.volume.minimumSize"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.volume.providers[0].purpose"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.volume.providers[0].name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.volume.providers[2].purpose"),
				})),
			))
		})

		It("should forbid Seed with overlapping networks", func() {
			shootDefaultPodCIDR := garden.CIDR("10.0.1.128/28")     // 10.0.1.128 -> 10.0.1.143
			shootDefaultServiceCIDR := garden.CIDR("10.0.1.144/30") // 10.0.1.144 -> 10.0.1.147
			// Pods CIDR overlaps with Nodes network
			// Services CIDR overlaps with Nodes and Pods
			// Shoot default pod CIDR overlaps with services
			// Shoot default pod CIDR overlaps with shoot default pod CIDR
			seed.Spec.Networks = garden.SeedNetworks{
				Nodes:    garden.CIDR("10.0.0.0/8"),   // 10.0.0.0 -> 10.255.255.255
				Pods:     garden.CIDR("10.0.1.0/24"),  // 10.0.1.0 -> 10.0.1.255
				Services: garden.CIDR("10.0.1.64/26"), // 10.0.1.64 -> 10.0.1.127
				ShootDefaults: &garden.ShootNetworks{
					Pods:     &shootDefaultPodCIDR,
					Services: &shootDefaultServiceCIDR,
				},
			}

			errorList := ValidateSeed(seed)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.networks.pods"),
				"Detail": Equal(`must not be a subset of "spec.networks.nodes" ("10.0.0.0/8")`),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.networks.services"),
				"Detail": Equal(`must not be a subset of "spec.networks.nodes" ("10.0.0.0/8")`),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.networks.shootDefaults.pods"),
				"Detail": Equal(`must not be a subset of "spec.networks.nodes" ("10.0.0.0/8")`),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.networks.shootDefaults.services"),
				"Detail": Equal(`must not be a subset of "spec.networks.nodes" ("10.0.0.0/8")`),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.networks.services"),
				"Detail": Equal(`must not be a subset of "spec.networks.pods" ("10.0.1.0/24")`),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.networks.shootDefaults.pods"),
				"Detail": Equal(`must not be a subset of "spec.networks.pods" ("10.0.1.0/24")`),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.networks.shootDefaults.services"),
				"Detail": Equal(`must not be a subset of "spec.networks.pods" ("10.0.1.0/24")`),
			}))
		})

		It("should fail updating immutable fields", func() {
			newSeed := prepareSeedForUpdate(seed)
			newSeed.Spec.Networks = garden.SeedNetworks{
				Nodes:    garden.CIDR("10.1.0.0/16"),
				Pods:     garden.CIDR("10.2.0.0/16"),
				Services: garden.CIDR("10.3.1.64/26"),
			}
			otherRegion := "other-region"
			newSeed.Spec.Backup.Provider = "other-provider"
			newSeed.Spec.Backup.Region = &otherRegion

			errorList := ValidateSeedUpdate(newSeed, seed)

			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.networks.pods"),
				"Detail": Equal(`field is immutable`),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.networks.services"),
				"Detail": Equal(`field is immutable`),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.networks.nodes"),
				"Detail": Equal(`field is immutable`),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.backup.region"),
				"Detail": Equal(`field is immutable`),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.backup.provider"),
				"Detail": Equal(`field is immutable`),
			}))
		})

		Context("#validateSeedBackupUpdate", func() {
			It("should allow adding backup profile", func() {
				seed.Spec.Backup = nil
				newSeed := prepareSeedForUpdate(seed)
				newSeed.Spec.Backup = backup

				errorList := ValidateSeedUpdate(newSeed, seed)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid removing backup profile", func() {
				newSeed := prepareSeedForUpdate(seed)
				newSeed.Spec.Backup = nil

				errorList := ValidateSeedUpdate(newSeed, seed)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.backup"),
					"Detail": Equal(`field is immutable`),
				}))
			})
		})
	})

	Describe("#ValidateQuota", func() {
		var quota *garden.Quota

		BeforeEach(func() {
			quota = &garden.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "quota-1",
					Namespace: "my-namespace",
				},
				Spec: garden.QuotaSpec{
					Scope: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
					},
					Metrics: corev1.ResourceList{
						"cpu":    resource.MustParse("200"),
						"memory": resource.MustParse("4000Gi"),
					},
				},
			}
		})

		It("should not return any errors", func() {
			errorList := ValidateQuota(quota)

			Expect(errorList).To(HaveLen(0))
		})

		It("should forbid Quota specification with empty or invalid keys", func() {
			quota.ObjectMeta = metav1.ObjectMeta{}
			quota.Spec.Scope = corev1.ObjectReference{}
			quota.Spec.Metrics["key"] = resource.MustParse("-100")

			errorList := ValidateQuota(quota)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.namespace"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.scope"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.metrics[key]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.metrics[key]"),
				})),
			))
		})
	})

	Describe("#ValidateSecretBinding, #ValidateSecretBindingUpdate", func() {
		var secretBinding *garden.SecretBinding

		BeforeEach(func() {
			secretBinding = &garden.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "profile",
					Namespace: "garden",
				},
				SecretRef: corev1.SecretReference{
					Name:      "my-secret",
					Namespace: "my-namespace",
				},
			}
		})

		It("should not return any errors", func() {
			errorList := ValidateSecretBinding(secretBinding)

			Expect(errorList).To(HaveLen(0))
		})

		It("should forbid empty SecretBinding resources", func() {
			secretBinding.ObjectMeta = metav1.ObjectMeta{}
			secretBinding.SecretRef = corev1.SecretReference{}

			errorList := ValidateSecretBinding(secretBinding)

			Expect(errorList).To(HaveLen(3))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.namespace"),
			}))
			Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("secretRef.name"),
			}))
		})

		It("should forbid empty stated Quota names", func() {
			secretBinding.Quotas = []corev1.ObjectReference{
				{},
			}

			errorList := ValidateSecretBinding(secretBinding)

			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("quotas[0].name"),
			}))
		})

		It("should forbid updating the secret binding spec", func() {
			newSecretBinding := prepareSecretBindingForUpdate(secretBinding)
			newSecretBinding.SecretRef.Name = "another-name"
			newSecretBinding.Quotas = append(newSecretBinding.Quotas, corev1.ObjectReference{
				Name:      "new-quota",
				Namespace: "new-quota-ns",
			})

			errorList := ValidateSecretBindingUpdate(newSecretBinding, secretBinding)

			Expect(errorList).To(HaveLen(2))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("secretRef"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("quotas"),
			}))
		})
	})

	Describe("#ValidateWorker", func() {
		DescribeTable("reject when maxUnavailable and maxSurge are invalid",
			func(maxUnavailable, maxSurge intstr.IntOrString, expectType field.ErrorType) {
				worker := garden.Worker{
					Name:           "worker-name",
					MachineType:    "large",
					MaxUnavailable: maxUnavailable,
					MaxSurge:       maxSurge,
				}
				errList := ValidateWorker(worker, nil)

				Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(expectType),
				}))))
			},

			// double zero values (percent or int)
			Entry("two zero integers", intstr.FromInt(0), intstr.FromInt(0), field.ErrorTypeInvalid),
			Entry("zero int and zero percent", intstr.FromInt(0), intstr.FromString("0%"), field.ErrorTypeInvalid),
			Entry("zero percent and zero int", intstr.FromString("0%"), intstr.FromInt(0), field.ErrorTypeInvalid),
			Entry("two zero percents", intstr.FromString("0%"), intstr.FromString("0%"), field.ErrorTypeInvalid),

			// greater than 100
			Entry("maxUnavailable greater than 100 percent", intstr.FromString("101%"), intstr.FromString("100%"), field.ErrorTypeInvalid),

			// below zero tests
			Entry("values are not below zero", intstr.FromInt(-1), intstr.FromInt(0), field.ErrorTypeInvalid),
			Entry("percentage is not less than zero", intstr.FromString("-90%"), intstr.FromString("90%"), field.ErrorTypeInvalid),
		)

		DescribeTable("reject when labels are invalid",
			func(labels map[string]string, expectType field.ErrorType) {
				worker := garden.Worker{
					Name:           "worker-name",
					MachineType:    "large",
					MaxSurge:       intstr.FromInt(1),
					MaxUnavailable: intstr.FromInt(0),
					Labels:         labels,
				}
				errList := ValidateWorker(worker, nil)

				Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(expectType),
				}))))
			},

			// invalid keys
			Entry("missing prefix", map[string]string{"/foo": "bar"}, field.ErrorTypeInvalid),
			Entry("too long name", map[string]string{"foo/somethingthatiswaylongerthanthelimitofthiswhichissixtythreecharacters": "baz"}, field.ErrorTypeInvalid),
			Entry("too many parts", map[string]string{"foo/bar/baz": "null"}, field.ErrorTypeInvalid),
			Entry("invalid name", map[string]string{"foo/bar%baz": "null"}, field.ErrorTypeInvalid),

			// invalid values
			Entry("too long", map[string]string{"foo": "somethingthatiswaylongerthanthelimitofthiswhichissixtythreecharacters"}, field.ErrorTypeInvalid),
			Entry("invalid", map[string]string{"foo": "no/slashes/allowed"}, field.ErrorTypeInvalid),
		)

		DescribeTable("reject when annotations are invalid",
			func(annotations map[string]string, expectType field.ErrorType) {
				worker := garden.Worker{
					Name:           "worker-name",
					MachineType:    "large",
					MaxSurge:       intstr.FromInt(1),
					MaxUnavailable: intstr.FromInt(0),
					Annotations:    annotations,
				}
				errList := ValidateWorker(worker, nil)

				Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(expectType),
				}))))
			},

			// invalid keys
			Entry("missing prefix", map[string]string{"/foo": "bar"}, field.ErrorTypeInvalid),
			Entry("too long name", map[string]string{"foo/somethingthatiswaylongerthanthelimitofthiswhichissixtythreecharacters": "baz"}, field.ErrorTypeInvalid),
			Entry("too many parts", map[string]string{"foo/bar/baz": "null"}, field.ErrorTypeInvalid),
			Entry("invalid name", map[string]string{"foo/bar%baz": "null"}, field.ErrorTypeInvalid),

			// invalid value
			Entry("too long", map[string]string{"foo": strings.Repeat("a", 262142)}, field.ErrorTypeTooLong),
		)

		DescribeTable("reject when taints are invalid",
			func(taints []corev1.Taint, expectType field.ErrorType) {
				worker := garden.Worker{
					Name:           "worker-name",
					MachineType:    "large",
					MaxSurge:       intstr.FromInt(1),
					MaxUnavailable: intstr.FromInt(0),
					Taints:         taints,
				}
				errList := ValidateWorker(worker, nil)

				Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(expectType),
				}))))
			},

			// invalid keys
			Entry("missing prefix", []corev1.Taint{{Key: "/foo", Value: "bar", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeInvalid),
			Entry("missing prefix", []corev1.Taint{{Key: "/foo", Value: "bar", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeInvalid),
			Entry("too long name", []corev1.Taint{{Key: "foo/somethingthatiswaylongerthanthelimitofthiswhichissixtythreecharacters", Value: "bar", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeInvalid),
			Entry("too many parts", []corev1.Taint{{Key: "foo/bar/baz", Value: "bar", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeInvalid),
			Entry("invalid name", []corev1.Taint{{Key: "foo/bar%baz", Value: "bar", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeInvalid),

			// invalid values
			Entry("too long", []corev1.Taint{{Key: "foo", Value: "somethingthatiswaylongerthanthelimitofthiswhichissixtythreecharacters", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeInvalid),
			Entry("invalid", []corev1.Taint{{Key: "foo", Value: "no/slashes/allowed", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeInvalid),

			// invalid effects
			Entry("no effect", []corev1.Taint{{Key: "foo", Value: "bar"}}, field.ErrorTypeRequired),
			Entry("non-existing", []corev1.Taint{{Key: "foo", Value: "bar", Effect: corev1.TaintEffect("does-not-exist")}}, field.ErrorTypeNotSupported),

			// uniqueness by key/effect
			Entry("not unique", []corev1.Taint{{Key: "foo", Value: "bar", Effect: corev1.TaintEffectNoSchedule}, {Key: "foo", Value: "baz", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeDuplicate),
		)
	})

	Describe("#ValidateWorkers", func() {
		DescribeTable("validate that at least one active worker pool is configured",
			func(min1, max1, min2, max2 int, matcher gomegatypes.GomegaMatcher) {
				workers := []garden.Worker{
					{
						AutoScalerMin: min1,
						AutoScalerMax: max1,
					},
					{
						AutoScalerMin: min2,
						AutoScalerMax: max2,
					},
				}

				errList := ValidateWorkers(workers, nil)

				Expect(errList).To(matcher)
			},

			Entry("at least one worker pool min>0, max>0", 0, 0, 1, 1, HaveLen(0)),
			Entry("all worker pools min=max=0", 0, 0, 0, 0, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(field.ErrorTypeForbidden),
			})))),
		)
	})

	Describe("#ValidateKubeletConfiguration", func() {
		validResourceQuantityValueMi := "100Mi"
		validResourceQuantityValueKi := "100"
		invalidResourceQuantityValue := "-100Mi"
		validPercentValue := "5%"
		invalidPercentValueLow := "-5%"
		invalidPercentValueHigh := "110%"
		invalidValue := "5X"

		DescribeTable("validate the kubelet configuration - EvictionHard & EvictionSoft",
			func(memoryAvailable, imagefsAvailable, imagefsInodesFree, nodefsAvailable, nodefsInodesFree string, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := garden.KubeletConfig{
					EvictionHard: &garden.KubeletConfigEviction{
						MemoryAvailable:   &memoryAvailable,
						ImageFSAvailable:  &imagefsAvailable,
						ImageFSInodesFree: &imagefsInodesFree,
						NodeFSAvailable:   &nodefsAvailable,
						NodeFSInodesFree:  &nodefsInodesFree,
					},
					EvictionSoft: &garden.KubeletConfigEviction{
						MemoryAvailable:   &memoryAvailable,
						ImageFSAvailable:  &imagefsAvailable,
						ImageFSInodesFree: &imagefsInodesFree,
						NodeFSAvailable:   &nodefsAvailable,
						NodeFSInodesFree:  &nodefsInodesFree,
					},
				}

				errList := ValidateKubeletConfig(kubeletConfig, nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", validResourceQuantityValueMi, validResourceQuantityValueKi, validPercentValue, validPercentValue, validPercentValue, HaveLen(0)),
			Entry("only allow resource.Quantity or percent value for any value", invalidValue, validPercentValue, validPercentValue, validPercentValue, validPercentValue, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionHard.memoryAvailable").String()),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionSoft.memoryAvailable").String()),
				})))),
			Entry("do not allow negative resource.Quantity", invalidResourceQuantityValue, validPercentValue, validPercentValue, validPercentValue, validPercentValue, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionHard.memoryAvailable").String()),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionSoft.memoryAvailable").String()),
				})))),
			Entry("do not allow negative percentages", invalidPercentValueLow, validPercentValue, validPercentValue, validPercentValue, validPercentValue, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionHard.memoryAvailable").String()),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionSoft.memoryAvailable").String()),
				})))),
			Entry("do not allow percentages > 100", invalidPercentValueHigh, validPercentValue, validPercentValue, validPercentValue, validPercentValue, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionHard.memoryAvailable").String()),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionSoft.memoryAvailable").String()),
				})))),
		)

		validResourceQuantity := resource.MustParse(validResourceQuantityValueMi)
		DescribeTable("validate the kubelet configuration - EvictionMinimumReclaim",
			func(memoryAvailable, imagefsAvailable, imagefsInodesFree, nodefsAvailable, nodefsInodesFree resource.Quantity, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := garden.KubeletConfig{
					EvictionMinimumReclaim: &garden.KubeletConfigEvictionMinimumReclaim{
						MemoryAvailable:   &memoryAvailable,
						ImageFSAvailable:  &imagefsAvailable,
						ImageFSInodesFree: &imagefsInodesFree,
						NodeFSAvailable:   &nodefsAvailable,
						NodeFSInodesFree:  &nodefsInodesFree,
					},
				}

				errList := ValidateKubeletConfig(kubeletConfig, nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", validResourceQuantity, validResourceQuantity, validResourceQuantity, validResourceQuantity, validResourceQuantity, HaveLen(0)),
			Entry("only allow positive resource.Quantity for any value", resource.MustParse(invalidResourceQuantityValue), validResourceQuantity, validResourceQuantity, validResourceQuantity, validResourceQuantity, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("evictionMinimumReclaim.memoryAvailable").String()),
			})))),
		)
		validDuration := metav1.Duration{Duration: 2 * time.Minute}
		invalidDuration := metav1.Duration{Duration: -2 * time.Minute}
		DescribeTable("validate the kubelet configuration - KubeletConfigEvictionSoftGracePeriod",
			func(memoryAvailable, imagefsAvailable, imagefsInodesFree, nodefsAvailable, nodefsInodesFree metav1.Duration, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := garden.KubeletConfig{
					EvictionSoftGracePeriod: &garden.KubeletConfigEvictionSoftGracePeriod{
						MemoryAvailable:   &memoryAvailable,
						ImageFSAvailable:  &imagefsAvailable,
						ImageFSInodesFree: &imagefsInodesFree,
						NodeFSAvailable:   &nodefsAvailable,
						NodeFSInodesFree:  &nodefsInodesFree,
					},
				}

				errList := ValidateKubeletConfig(kubeletConfig, nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", validDuration, validDuration, validDuration, validDuration, validDuration, HaveLen(0)),
			Entry("only allow positive Duration for any value", invalidDuration, validDuration, validDuration, validDuration, validDuration, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionSoftGracePeriod.memoryAvailable").String()),
				})))),
		)

		DescribeTable("validate the kubelet configuration - EvictionPressureTransitionPeriod",
			func(evictionPressureTransitionPeriod metav1.Duration, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := garden.KubeletConfig{
					EvictionPressureTransitionPeriod: &evictionPressureTransitionPeriod,
				}

				errList := ValidateKubeletConfig(kubeletConfig, nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", validDuration, HaveLen(0)),
			Entry("only allow positive Duration", invalidDuration, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionPressureTransitionPeriod").String()),
				})))),
		)

		DescribeTable("validate the kubelet configuration - EvictionMaxPodGracePeriod",
			func(evictionMaxPodGracePeriod int32, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := garden.KubeletConfig{
					EvictionMaxPodGracePeriod: &evictionMaxPodGracePeriod,
				}

				errList := ValidateKubeletConfig(kubeletConfig, nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", int32(90), HaveLen(0)),
			Entry("only allow positive number", int32(-3), ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionMaxPodGracePeriod").String()),
				})))),
		)

		DescribeTable("validate the kubelet configuration - MaxPods",
			func(maxPods int32, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := garden.KubeletConfig{
					MaxPods: &maxPods,
				}

				errList := ValidateKubeletConfig(kubeletConfig, nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", int32(110), HaveLen(0)),
			Entry("only allow positive number", int32(-3), ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("maxPods").String()),
				})))),
		)
	})

	Describe("#ValidateHibernationSchedules", func() {
		DescribeTable("validate hibernation schedules",
			func(schedules []garden.HibernationSchedule, matcher gomegatypes.GomegaMatcher) {
				Expect(ValidateHibernationSchedules(schedules, nil)).To(matcher)
			},
			Entry("valid schedules", []garden.HibernationSchedule{{Start: makeStringPointer("1 * * * *"), End: makeStringPointer("2 * * * *")}}, BeEmpty()),
			Entry("nil schedules", nil, BeEmpty()),
			Entry("duplicate start and end value in same schedule",
				[]garden.HibernationSchedule{{Start: makeStringPointer("* * * * *"), End: makeStringPointer("* * * * *")}},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeDuplicate),
				})))),
			Entry("duplicate start and end value in different schedules",
				[]garden.HibernationSchedule{{Start: makeStringPointer("1 * * * *"), End: makeStringPointer("2 * * * *")}, {Start: makeStringPointer("1 * * * *"), End: makeStringPointer("3 * * * *")}},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeDuplicate),
				})))),
			Entry("invalid schedule",
				[]garden.HibernationSchedule{{Start: makeStringPointer("foo"), End: makeStringPointer("* * * * *")}},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeInvalid),
				})))),
		)
	})

	Describe("#ValidateHibernationCronSpec", func() {
		DescribeTable("validate cron spec",
			func(seenSpecs sets.String, spec string, matcher gomegatypes.GomegaMatcher) {
				Expect(ValidateHibernationCronSpec(seenSpecs, spec, nil)).To(matcher)
			},
			Entry("invalid spec", sets.NewString(), "foo", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(field.ErrorTypeInvalid),
			})))),
			Entry("duplicate spec", sets.NewString("* * * * *"), "* * * * *", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(field.ErrorTypeDuplicate),
			})))),
		)

		It("should add the inspected cron spec to the set if there were no issues", func() {
			var (
				s    = sets.NewString()
				spec = "* * * * *"
			)
			Expect(ValidateHibernationCronSpec(s, spec, nil)).To(BeEmpty())
			Expect(s.Has(spec)).To(BeTrue())
		})

		It("should not add the inspected cron spec to the set if there were issues", func() {
			var (
				s    = sets.NewString()
				spec = "foo"
			)
			Expect(ValidateHibernationCronSpec(s, spec, nil)).NotTo(BeEmpty())
			Expect(s.Has(spec)).To(BeFalse())
		})
	})

	Describe("#ValidateHibernationScheduleLocation", func() {
		DescribeTable("validate hibernation schedule location",
			func(location string, matcher gomegatypes.GomegaMatcher) {
				Expect(ValidateHibernationScheduleLocation(location, nil)).To(matcher)
			},
			Entry("utc location", "UTC", BeEmpty()),
			Entry("empty location -> utc", "", BeEmpty()),
			Entry("invalid location", "should not exist", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(field.ErrorTypeInvalid),
			})))),
		)
	})

	Describe("#ValidateHibernationSchedule", func() {
		DescribeTable("validate schedule",
			func(seenSpecs sets.String, schedule *garden.HibernationSchedule, matcher gomegatypes.GomegaMatcher) {
				errList := ValidateHibernationSchedule(seenSpecs, schedule, nil)
				Expect(errList).To(matcher)
			},

			Entry("valid schedule", sets.NewString(), &garden.HibernationSchedule{Start: makeStringPointer("1 * * * *"), End: makeStringPointer("2 * * * *")}, BeEmpty()),
			Entry("invalid start value", sets.NewString(), &garden.HibernationSchedule{Start: makeStringPointer(""), End: makeStringPointer("* * * * *")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("start").String()),
			})))),
			Entry("invalid end value", sets.NewString(), &garden.HibernationSchedule{Start: makeStringPointer("* * * * *"), End: makeStringPointer("")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("end").String()),
			})))),
			Entry("invalid location", sets.NewString(), &garden.HibernationSchedule{Start: makeStringPointer("1 * * * *"), End: makeStringPointer("2 * * * *"), Location: makeStringPointer("foo")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("location").String()),
			})))),
			Entry("equal start and end value", sets.NewString(), &garden.HibernationSchedule{Start: makeStringPointer("* * * * *"), End: makeStringPointer("* * * * *")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal(field.NewPath("end").String()),
			})))),
			Entry("nil start", sets.NewString(), &garden.HibernationSchedule{End: makeStringPointer("* * * * *")}, BeEmpty()),
			Entry("nil end", sets.NewString(), &garden.HibernationSchedule{Start: makeStringPointer("* * * * *")}, BeEmpty()),
			Entry("start and end nil", sets.NewString(), &garden.HibernationSchedule{},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeRequired),
				})))),
			Entry("invalid start and end value", sets.NewString(), &garden.HibernationSchedule{Start: makeStringPointer(""), End: makeStringPointer("")},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(field.NewPath("start").String()),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(field.NewPath("end").String()),
					})),
				)),
		)
	})

	Describe("#ValidateShoot, #ValidateShootUpdate", func() {
		var (
			shoot *garden.Shoot

			domain      = "my-cluster.example.com"
			dnsProvider = "some-provider"

			nodeCIDR    = garden.CIDR("10.250.0.0/16")
			podCIDR     = garden.CIDR("100.96.0.0/11")
			serviceCIDR = garden.CIDR("100.64.0.0/13")
			invalidCIDR = garden.CIDR("invalid-cidr")
			vpcCIDR     = garden.CIDR("10.0.0.0/8")
			addon       = garden.Addon{
				Enabled: true,
			}
			k8sNetworks = garden.K8SNetworks{
				Nodes:    &nodeCIDR,
				Pods:     &podCIDR,
				Services: &serviceCIDR,
			}
			invalidK8sNetworks = garden.K8SNetworks{
				Nodes:    &invalidCIDR,
				Pods:     &invalidCIDR,
				Services: &invalidCIDR,
			}
			worker = garden.Worker{
				Name:           "worker-name",
				MachineType:    "large",
				AutoScalerMin:  1,
				AutoScalerMax:  1,
				MaxSurge:       intstr.FromInt(1),
				MaxUnavailable: intstr.FromInt(0),
			}
			invalidWorker = garden.Worker{
				Name:           "",
				MachineType:    "",
				AutoScalerMin:  -1,
				AutoScalerMax:  -2,
				MaxSurge:       intstr.FromInt(1),
				MaxUnavailable: intstr.FromInt(0),
			}
			invalidWorkerName = garden.Worker{
				Name:           "not_compliant",
				MachineType:    "large",
				AutoScalerMin:  1,
				AutoScalerMax:  1,
				MaxSurge:       intstr.FromInt(1),
				MaxUnavailable: intstr.FromInt(0),
			}
			invalidWorkerTooLongName = garden.Worker{
				Name:           "worker-name-is-too-long",
				MachineType:    "large",
				AutoScalerMin:  1,
				AutoScalerMax:  1,
				MaxSurge:       intstr.FromInt(1),
				MaxUnavailable: intstr.FromInt(0),
			}
			workerAutoScalingInvalid = garden.Worker{
				Name:           "cpu-worker",
				MachineType:    "large",
				AutoScalerMin:  0,
				AutoScalerMax:  2,
				MaxSurge:       intstr.FromInt(1),
				MaxUnavailable: intstr.FromInt(0),
			}
			workerAutoScalingMinMaxZero = garden.Worker{
				Name:           "cpu-worker",
				MachineType:    "large",
				AutoScalerMin:  0,
				AutoScalerMax:  0,
				MaxSurge:       intstr.FromInt(1),
				MaxUnavailable: intstr.FromInt(0),
			}
		)

		BeforeEach(func() {
			shoot = &garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "my-namespace",
				},
				Spec: garden.ShootSpec{
					Addons: &garden.Addons{
						Kube2IAM: &garden.Kube2IAM{
							Addon: addon,
							Roles: []garden.Kube2IAMRole{
								{
									Name:        "iam-role",
									Description: "some-text",
									Policy:      `{"some-valid": "json-document"}`,
								},
							},
						},
						KubernetesDashboard: &garden.KubernetesDashboard{
							Addon: addon,
						},
						ClusterAutoscaler: &garden.AddonClusterAutoscaler{
							Addon: addon,
						},
						NginxIngress: &garden.NginxIngress{
							Addon: addon,
						},
						KubeLego: &garden.KubeLego{
							Addon: addon,
							Mail:  "info@example.com",
						},
					},
					Cloud: garden.Cloud{
						Profile: "aws-profile",
						Region:  "eu-west-1",
						SecretBindingRef: corev1.LocalObjectReference{
							Name: "my-secret",
						},
						AWS: &garden.AWSCloud{
							Networks: garden.AWSNetworks{
								K8SNetworks: k8sNetworks,
								Internal:    []garden.CIDR{"10.250.1.0/24"},
								Public:      []garden.CIDR{"10.250.2.0/24"},
								Workers:     []garden.CIDR{"10.250.3.0/24"},
								VPC: garden.AWSVPC{
									CIDR: &nodeCIDR,
								},
							},
							Workers: []garden.AWSWorker{
								{
									Worker:     worker,
									VolumeSize: "20Gi",
									VolumeType: "default",
								},
							},
							Zones: []string{"eu-west-1a"},
						},
					},
					DNS: garden.DNS{
						Provider: &dnsProvider,
						Domain:   &domain,
					},
					Kubernetes: garden.Kubernetes{
						Version: "1.11.2",
						KubeAPIServer: &garden.KubeAPIServerConfig{
							OIDCConfig: &garden.OIDCConfig{
								CABundle:       makeStringPointer("-----BEGIN CERTIFICATE-----\nMIICRzCCAfGgAwIBAgIJALMb7ecMIk3MMA0GCSqGSIb3DQEBCwUAMH4xCzAJBgNV\nBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNVBAcMBkxvbmRvbjEYMBYGA1UE\nCgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1JVCBEZXBhcnRtZW50MRswGQYD\nVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTAwIBcNMTcwNDI2MjMyNjUyWhgPMjExNzA0\nMDIyMzI2NTJaMH4xCzAJBgNVBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNV\nBAcMBkxvbmRvbjEYMBYGA1UECgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1J\nVCBEZXBhcnRtZW50MRswGQYDVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTAwXDANBgkq\nhkiG9w0BAQEFAANLADBIAkEAtBMa7NWpv3BVlKTCPGO/LEsguKqWHBtKzweMY2CV\ntAL1rQm913huhxF9w+ai76KQ3MHK5IVnLJjYYA5MzP2H5QIDAQABo1AwTjAdBgNV\nHQ4EFgQU22iy8aWkNSxv0nBxFxerfsvnZVMwHwYDVR0jBBgwFoAU22iy8aWkNSxv\n0nBxFxerfsvnZVMwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAANBAEOefGbV\nNcHxklaW06w6OBYJPwpIhCVozC1qdxGX1dg8VkEKzjOzjgqVD30m59OFmSlBmHsl\nnkVA6wyOSDYBf3o=\n-----END CERTIFICATE-----"),
								ClientID:       makeStringPointer("client-id"),
								GroupsClaim:    makeStringPointer("groups-claim"),
								GroupsPrefix:   makeStringPointer("groups-prefix"),
								IssuerURL:      makeStringPointer("https://some-endpoint.com"),
								UsernameClaim:  makeStringPointer("user-claim"),
								UsernamePrefix: makeStringPointer("user-prefix"),
							},
							AdmissionPlugins: []garden.AdmissionPlugin{
								{
									Name: "PodNodeSelector",
									Config: makeStringPointer(`podNodeSelectorPluginConfig:
  clusterDefaultNodeSelector: <node-selectors-labels>
  namespace1: <node-selectors-labels>
  namespace2: <node-selectors-labels>`),
								},
							},
							AuditConfig: &garden.AuditConfig{
								AuditPolicy: &garden.AuditPolicy{
									ConfigMapRef: &corev1.LocalObjectReference{
										Name: "audit-policy-config",
									},
								},
							},
							EnableBasicAuthentication: makeBoolPointer(true),
						},
						KubeControllerManager: &garden.KubeControllerManagerConfig{
							NodeCIDRMaskSize: makeIntPointer(22),
							HorizontalPodAutoscalerConfig: &garden.HorizontalPodAutoscalerConfig{
								DownscaleDelay: makeDurationPointer(15 * time.Minute),
								SyncPeriod:     makeDurationPointer(30 * time.Second),
								Tolerance:      makeFloat64Pointer(0.1),
								UpscaleDelay:   makeDurationPointer(1 * time.Minute),
							},
						},
					},
					Networking: &garden.Networking{
						Type: "some-network-plugin",
					},
					Maintenance: &garden.Maintenance{
						AutoUpdate: &garden.MaintenanceAutoUpdate{
							KubernetesVersion: true,
						},
						TimeWindow: &garden.MaintenanceTimeWindow{
							Begin: "220000+0100",
							End:   "230000+0100",
						},
					},
				},
			}
		})

		It("should forbid shoots containing two consecutive hyphens", func() {
			shoot.ObjectMeta.Name = "sho--ot"

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("metadata.name"),
			}))
		})

		It("should forbid shoots with a not DNS-1123 label compliant name", func() {
			shoot.ObjectMeta.Name = "shoot.test"

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("metadata.name"),
			}))
		})

		It("should forbid empty Shoot resources", func() {
			shoot := &garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       garden.ShootSpec{},
			}

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(HaveLen(3))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.namespace"),
			}))
			Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.cloud.aws/azure/gcp/alicloud/openstack/packet"),
			}))
		})

		It("should forbid unsupported addon configuration", func() {
			shoot.Spec.Addons.Kube2IAM.Roles = []garden.Kube2IAMRole{
				{
					Name:        "",
					Description: "",
					Policy:      "invalid-json",
				},
			}
			shoot.Spec.Addons.KubeLego.Mail = "some-invalid-email"
			shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode = makeStringPointer("does-not-exist")

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.addons.kube2iam.roles[0].name"),
			})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.addons.kube2iam.roles[0].description"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.addons.kube2iam.roles[0].policy"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.addons.kube-lego.mail"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.addons.kubernetes-dashboard.authenticationMode"),
				})),
			))
		})

		It("should forbid using basic auth mode for kubernetes dashboard when it's disabled in kube-apiserver config", func() {
			shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode = makeStringPointer(garden.KubernetesDashboardAuthModeBasic)
			shoot.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = makeBoolPointer(false)

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.addons.kubernetes-dashboard.authenticationMode"),
			}))))
		})

		It("should allow using basic auth mode for kubernetes dashboard when it's enabled in kube-apiserver config", func() {
			shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode = makeStringPointer(garden.KubernetesDashboardAuthModeBasic)
			shoot.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = makeBoolPointer(true)

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid unsupported cloud specification (provider independent)", func() {
			shoot.Spec.Cloud.Profile = ""
			shoot.Spec.Cloud.Region = ""
			shoot.Spec.Cloud.SecretBindingRef = corev1.LocalObjectReference{
				Name: "",
			}
			shoot.Spec.Cloud.Seed = makeStringPointer("")

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(HaveLen(4))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.cloud.profile"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.cloud.region"),
			}))
			Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.cloud.secretBindingRef.name"),
			}))
			Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.cloud.seed"),
			}))
		})

		It("should forbid updating some cloud keys", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.Cloud.Profile = "another-profile"
			newShoot.Spec.Cloud.Region = "another-region"
			newShoot.Spec.Cloud.SecretBindingRef = corev1.LocalObjectReference{
				Name: "another-reference",
			}

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.cloud.profile"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.cloud.region"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.cloud.secretBindingRef"),
				}))),
			)
		})

		It("should forbid updating the seed, if it has been set previously", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.Cloud.Seed = makeStringPointer("another-seed")
			shoot.Spec.Cloud.Seed = makeStringPointer("first-seed")

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.cloud.seed"),
				}))),
			)
		})

		It("should forbid passing an extension w/o type information", func() {
			extension := garden.Extension{}
			shoot.Spec.Extensions = append(shoot.Spec.Extensions, extension)

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.extensions[0].type"),
				}))))
		})

		It("should allow passing an extension w/ type information", func() {
			extension := garden.Extension{
				Type: "arbitrary",
			}
			shoot.Spec.Extensions = append(shoot.Spec.Extensions, extension)

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow updating the seed if it has not been set previously", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.Cloud.Seed = makeStringPointer("another-seed")
			shoot.Spec.Cloud.Seed = nil

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(HaveLen(0))
		})

		Context("AWS specific validation", func() {
			var (
				fldPath  = "aws"
				awsCloud *garden.AWSCloud
			)

			BeforeEach(func() {
				awsCloud = &garden.AWSCloud{
					Networks: garden.AWSNetworks{
						K8SNetworks: k8sNetworks,
						Internal:    []garden.CIDR{"10.250.1.0/24"},
						Public:      []garden.CIDR{"10.250.2.0/24"},
						Workers:     []garden.CIDR{"10.250.3.0/24"},
						VPC: garden.AWSVPC{
							CIDR: &vpcCIDR,
						},
					},
					Workers: []garden.AWSWorker{
						{
							Worker:     worker,
							VolumeSize: "20Gi",
							VolumeType: "default",
						},
					},
					Zones: []string{"eu-west-1a"},
				}
				shoot.Spec.Cloud.AWS = awsCloud
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			Context("CIDR", func() {

				It("should forbid invalid VPC CIDRs", func() {
					shoot.Spec.Cloud.AWS.Networks.VPC.CIDR = &invalidCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.vpc.cidr"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid invalid internal CIDR", func() {
					shoot.Spec.Cloud.AWS.Networks.Internal = []garden.CIDR{invalidCIDR}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.internal[0]"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid invalid public CIDR", func() {
					shoot.Spec.Cloud.AWS.Networks.Public = []garden.CIDR{invalidCIDR}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.public[0]"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid invalid workers CIDR", func() {
					shoot.Spec.Cloud.AWS.Networks.Workers = []garden.CIDR{invalidCIDR}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.workers[0]"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid internal CIDR which is not in VPC CIDR", func() {
					shoot.Spec.Cloud.AWS.Networks.Internal = []garden.CIDR{"1.1.1.1/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.internal[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.aws.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid public CIDR which is not in VPC CIDR", func() {
					shoot.Spec.Cloud.AWS.Networks.Public = []garden.CIDR{"1.1.1.1/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.public[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.aws.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid workers CIDR which are not in VPC and Nodes CIDR", func() {
					shoot.Spec.Cloud.AWS.Networks.Workers = []garden.CIDR{garden.CIDR("1.1.1.1/32")}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.workers[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.aws.networks.nodes" ("10.250.0.0/16")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.workers[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.aws.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid Pod CIDR to overlap with VPC CIDR", func() {
					podCIDR := garden.CIDR("10.0.0.1/32")
					shoot.Spec.Cloud.AWS.Networks.K8SNetworks.Pods = &podCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.pods"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid Services CIDR to overlap with VPC CIDR", func() {
					servicesCIDR := garden.CIDR("10.0.0.1/32")
					shoot.Spec.Cloud.AWS.Networks.K8SNetworks.Services = &servicesCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.services"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid VPC CIDRs to overlap with other VPC CIDRs", func() {
					overlappingCIDR := garden.CIDR("10.250.0.1/32")
					shoot.Spec.Cloud.AWS.Networks.Public = []garden.CIDR{overlappingCIDR}
					shoot.Spec.Cloud.AWS.Networks.Internal = []garden.CIDR{overlappingCIDR}
					shoot.Spec.Cloud.AWS.Networks.Workers = []garden.CIDR{overlappingCIDR}
					shoot.Spec.Cloud.AWS.Networks.Nodes = &overlappingCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.public[0]"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.internal[0]" ("10.250.0.1/32")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.workers[0]"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.internal[0]" ("10.250.0.1/32")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.internal[0]"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.public[0]" ("10.250.0.1/32")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.workers[0]"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.public[0]" ("10.250.0.1/32")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.internal[0]"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.workers[0]" ("10.250.0.1/32")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.public[0]"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.workers[0]" ("10.250.0.1/32")`),
					}))
				})

				It("should invalid k8s networks", func() {
					shoot.Spec.Cloud.AWS.Networks.K8SNetworks = invalidK8sNetworks

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.nodes"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.pods"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.services"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

			})

			It("should forbid non canonical CIDRs", func() {
				vpcCIDR := garden.CIDR("10.0.0.3/8")
				nodeCIDR := garden.CIDR("10.250.0.3/16")
				podCIDR := garden.CIDR("100.96.0.4/11")
				serviceCIDR := garden.CIDR("100.64.0.5/13")

				shoot.Spec.Cloud.AWS.Networks.Public = []garden.CIDR{"10.250.2.7/24"}
				shoot.Spec.Cloud.AWS.Networks.Internal = []garden.CIDR{"10.250.1.6/24"}
				shoot.Spec.Cloud.AWS.Networks.Workers = []garden.CIDR{"10.250.3.8/24"}
				shoot.Spec.Cloud.AWS.Networks.Nodes = &nodeCIDR
				shoot.Spec.Cloud.AWS.Networks.Services = &serviceCIDR
				shoot.Spec.Cloud.AWS.Networks.Pods = &podCIDR
				shoot.Spec.Cloud.AWS.Networks.VPC = garden.AWSVPC{CIDR: &vpcCIDR}

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(7))

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.aws.networks.vpc.cidr"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.aws.nodes"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.aws.pods"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.aws.services"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.aws.networks.internal[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.aws.networks.public[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.aws.networks.workers[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.AWSWorker{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
				}))
			})

			It("should enforce unique worker names", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.AWSWorker{
					{
						Worker:     worker,
						VolumeSize: "20Gi",
						VolumeType: "default",
					},
					{
						Worker:     worker,
						VolumeSize: "20Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[1]", fldPath)),
				}))
			})

			It("should forbid invalid worker configuration", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.AWSWorker{
					{
						Worker:     invalidWorker,
						VolumeSize: "hugo",
						VolumeType: "",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(7))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].machineType", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMin", fldPath)),
				}))
				Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMax", fldPath)),
				}))
				Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMax", fldPath)),
				}))
				Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeSize", fldPath)),
				}))
				Expect(*errorList[6]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeType", fldPath)),
				}))
			})

			It("should enforce workers min > 0 if max > 0", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.AWSWorker{
					{
						Worker:     workerAutoScalingInvalid,
						VolumeSize: "20Gi",
						VolumeType: "default",
					},
					{
						Worker:     worker,
						VolumeSize: "40Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMin", fldPath)),
				}))
			})

			It("should allow workers having min=max=0 if at least one pool is active", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.AWSWorker{
					{
						Worker:     worker,
						VolumeSize: "40Gi",
						VolumeType: "default",
					},
					{
						Worker:     workerAutoScalingMinMaxZero,
						VolumeSize: "20Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid worker pools with too less volume size", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.AWSWorker{
					{
						Worker:     worker,
						VolumeSize: "10Gi",
						VolumeType: "gp2",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeSize", fldPath)),
				}))
			})

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.AWS.Workers[0].Worker = invalidWorkerTooLongName

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid worker pools with names that are not DNS-1123 label compliant", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.AWSWorker{
					{
						Worker:     invalidWorkerName,
						VolumeSize: "20Gi",
						VolumeType: "gp2",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid an empty zones list", func() {
				shoot.Spec.Cloud.AWS.Zones = []string{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid updating networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.AWS.Networks.Workers[0] = garden.CIDR("10.250.0.0/24")
				newShoot.Spec.Cloud.AWS.Zones = []string{"another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})

			It("should forbid removing the AWS section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.AWS = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(HaveLen(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.cloud.aws/azure/gcp/alicloud/openstack/packet"),
				}))
			})

			Context("NodeCIDRMask validation", func() {
				var (
					defaultMaxPod           int32 = 110
					maxPod                  int32 = 260
					defaultNodeCIDRMaskSize       = 24
					testWorker              garden.Worker
				)

				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = &defaultNodeCIDRMaskSize
					shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &defaultMaxPod}
					testWorker = *worker.DeepCopy()
					testWorker.Name = "testworker"
				})

				It("should not return any errors", func() {
					worker.Kubelet = &garden.KubeletConfig{
						MaxPods: &defaultMaxPod,
					}
					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(0))
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							shoot.Spec.Cloud.AWS.Workers = append(shoot.Spec.Cloud.AWS.Workers, garden.AWSWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     testWorker,
							})

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))

							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
					Context("multiple worker pools", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							secondTestWorker := *testWorker.DeepCopy()
							secondTestWorker.Name = "testworker2"
							secondTestWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							shoot.Spec.Cloud.AWS.Workers = append(shoot.Spec.Cloud.AWS.Workers, garden.AWSWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     testWorker,
							}, garden.AWSWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     secondTestWorker,
							})

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})

					Context("Global default max pod", func() {
						It("should deny NodeCIDR with too few ips", func() {
							shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &maxPod}

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
				})
			})
		})

		Context("Azure specific validation", func() {
			var (
				fldPath    = "azure"
				azureCloud *garden.AzureCloud
			)

			BeforeEach(func() {
				azureCloud = &garden.AzureCloud{
					Networks: garden.AzureNetworks{
						K8SNetworks: k8sNetworks,
						Workers:     garden.CIDR("10.250.3.0/24"),
						VNet: garden.AzureVNet{
							CIDR: &vpcCIDR,
						},
					},
					Workers: []garden.AzureWorker{
						{
							Worker:     worker,
							VolumeSize: "35Gi",
							VolumeType: "default",
						},
					},
				}
				shoot.Spec.Cloud.AWS = nil
				shoot.Spec.Cloud.Azure = azureCloud
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid specifying a resource group configuration", func() {
				shoot.Spec.Cloud.Azure.ResourceGroup = &garden.AzureResourceGroup{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.resourceGroup.name", fldPath)),
				}))
			})

			It("should forbid specifying a vnet name", func() {
				shoot.Spec.Cloud.Azure.Networks.VNet = garden.AzureVNet{
					Name: makeStringPointer("existing-vnet"),
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.vnet.name", fldPath)),
				}))
			})

			Context("CIDR", func() {

				It("should forbid invalid VNet CIDRs", func() {
					shoot.Spec.Cloud.Azure.Networks.VNet.CIDR = &invalidCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.vnet.cidr"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid invalid workers CIDR", func() {
					shoot.Spec.Cloud.Azure.Networks.Workers = invalidCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.workers"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid workers which are not in VNet anmd Nodes CIDR", func() {
					notOverlappingCIDR := garden.CIDR("1.1.1.1/32")
					// shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Nodes = &notOverlappingCIDR
					shoot.Spec.Cloud.Azure.Networks.Workers = notOverlappingCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.workers"),
						"Detail": Equal(`must be a subset of "spec.cloud.azure.networks.nodes" ("10.250.0.0/16")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.workers"),
						"Detail": Equal(`must be a subset of "spec.cloud.azure.networks.vnet.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid Pod CIDR to overlap with VNet CIDR", func() {
					podCIDR := garden.CIDR("10.0.0.1/32")
					shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Pods = &podCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.pods"),
						"Detail": Equal(`must not be a subset of "spec.cloud.azure.networks.vnet.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid Services CIDR to overlap with VNet CIDR", func() {
					servicesCIDR := garden.CIDR("10.0.0.1/32")
					shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Services = &servicesCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.services"),
						"Detail": Equal(`must not be a subset of "spec.cloud.azure.networks.vnet.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should invalid k8s networks", func() {
					shoot.Spec.Cloud.Azure.Networks.K8SNetworks = invalidK8sNetworks

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.nodes"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.pods"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.services"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})
			})

			It("should forbid non canonical CIDRs", func() {
				vpcCIDR := garden.CIDR("10.0.0.3/8")
				nodeCIDR := garden.CIDR("10.250.0.3/16")
				podCIDR := garden.CIDR("100.96.0.4/11")
				serviceCIDR := garden.CIDR("100.64.0.5/13")
				workers := garden.CIDR("10.250.3.8/24")

				shoot.Spec.Cloud.Azure.Networks.Workers = workers
				shoot.Spec.Cloud.Azure.Networks.Nodes = &nodeCIDR
				shoot.Spec.Cloud.Azure.Networks.Services = &serviceCIDR
				shoot.Spec.Cloud.Azure.Networks.Pods = &podCIDR
				shoot.Spec.Cloud.Azure.Networks.VNet = garden.AzureVNet{CIDR: &vpcCIDR}

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(5))

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.azure.networks.vnet.cidr"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.azure.nodes"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.azure.pods"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.azure.services"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.azure.networks.workers[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.AzureWorker{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
				}))
			})

			It("should enforce unique worker names", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.AzureWorker{
					{
						Worker:     worker,
						VolumeSize: "35Gi",
						VolumeType: "default",
					},
					{
						Worker:     worker,
						VolumeSize: "35Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[1]", fldPath)),
				}))
			})

			It("should forbid invalid worker configuration", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.AzureWorker{
					{
						Worker:     invalidWorker,
						VolumeSize: "hugo",
						VolumeType: "",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(7))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].machineType", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMin", fldPath)),
				}))
				Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMax", fldPath)),
				}))
				Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMax", fldPath)),
				}))
				Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeSize", fldPath)),
				}))
				Expect(*errorList[6]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeType", fldPath)),
				}))
			})

			It("should enforce workers min > 0 if max > 0", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.AzureWorker{
					{
						Worker:     workerAutoScalingInvalid,
						VolumeSize: "40Gi",
						VolumeType: "default",
					},
					{
						Worker:     worker,
						VolumeSize: "40Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMin", fldPath)),
				}))
			})

			It("should allow workers having min=max=0 if at least one pool is active", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.AzureWorker{
					{
						Worker:     worker,
						VolumeSize: "40Gi",
						VolumeType: "default",
					},
					{
						Worker:     workerAutoScalingMinMaxZero,
						VolumeSize: "40Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid worker pools with too less volume size", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.AzureWorker{
					{
						Worker:     worker,
						VolumeSize: "30Gi",
						VolumeType: "gp2",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeSize", fldPath)),
				}))
			})

			It("should forbid worker volume sizes smaller than 35Gi", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.AzureWorker{
					{
						Worker:     worker,
						VolumeSize: "34Gi",
						VolumeType: "ok",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeSize", fldPath)),
				}))
			})

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.Azure.Workers[0].Worker = invalidWorkerTooLongName

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid worker pools with names that are not DNS-1123 label compliant", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.AzureWorker{
					{
						Worker:     invalidWorkerName,
						VolumeSize: "35Gi",
						VolumeType: "ok",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid updating resource group and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				cidr := garden.CIDR("10.250.0.0/19")
				newShoot.Spec.Cloud.Azure.Networks.Nodes = &cidr
				newShoot.Spec.Cloud.Azure.ResourceGroup = &garden.AzureResourceGroup{
					Name: "another-group",
				}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.resourceGroup", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.nodes", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.resourceGroup.name", fldPath)),
					})),
				))
			})

			It("should forbid removing the Azure section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.Azure = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(HaveLen(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.cloud.aws/azure/gcp/alicloud/openstack/packet"),
				}))
			})

			Context("NodeCIDRMask validation", func() {
				var (
					defaultMaxPod           int32 = 110
					maxPod                  int32 = 260
					defaultNodeCIDRMaskSize       = 24
					testWorker              garden.Worker
				)

				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = &defaultNodeCIDRMaskSize
					shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &defaultMaxPod}
					testWorker = *worker.DeepCopy()
					testWorker.Name = "testworker"
				})

				It("should not return any errors", func() {
					worker.Kubelet = &garden.KubeletConfig{
						MaxPods: &defaultMaxPod,
					}
					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(0))
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							shoot.Spec.Cloud.Azure.Workers = append(shoot.Spec.Cloud.Azure.Workers, garden.AzureWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     testWorker,
							})

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))

							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
					Context("multiple worker pools", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							secondTestWorker := *testWorker.DeepCopy()
							secondTestWorker.Name = "testworker2"
							secondTestWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							shoot.Spec.Cloud.Azure.Workers = append(shoot.Spec.Cloud.Azure.Workers, garden.AzureWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     testWorker,
							}, garden.AzureWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     secondTestWorker,
							})

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})

					Context("Global default max pod", func() {
						It("should deny NodeCIDR with too few ips", func() {
							shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &maxPod}

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
				})
			})
		})

		Context("GCP specific validation", func() {
			var (
				fldPath  = "gcp"
				gcpCloud *garden.GCPCloud
				internal = garden.CIDR("10.10.0.0/24")
			)

			BeforeEach(func() {
				gcpCloud = &garden.GCPCloud{
					Networks: garden.GCPNetworks{
						K8SNetworks: k8sNetworks,
						Internal:    &internal,
						Workers:     []garden.CIDR{"10.250.0.0/16"},
						VPC: &garden.GCPVPC{
							Name: "hugo",
						},
					},
					Workers: []garden.GCPWorker{
						{
							Worker:     worker,
							VolumeSize: "20Gi",
							VolumeType: "default",
						},
					},
					Zones: []string{"europe-west1-b"},
				}
				shoot.Spec.Cloud.AWS = nil
				shoot.Spec.Cloud.GCP = gcpCloud
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)
				Expect(errorList).To(BeEmpty())
			})

			Context("CIDR", func() {
				It("should forbid more than one CIDR", func() {
					shoot.Spec.Cloud.GCP.Networks.Workers = []garden.CIDR{"10.250.0.1/32", "10.250.0.2/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.workers"),
						"Detail": Equal("must specify only one worker cidr"),
					}))
				})

				It("should forbid invalid workers CIDR", func() {
					shoot.Spec.Cloud.GCP.Networks.Workers = []garden.CIDR{invalidCIDR}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.workers[0]"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid invalid internal CIDR", func() {
					invalidCIDR = garden.CIDR("invalid-cidr")
					shoot.Spec.Cloud.GCP.Networks.Internal = &invalidCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.internal"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid workers CIDR which are not in Nodes CIDR", func() {
					shoot.Spec.Cloud.GCP.Networks.Workers = []garden.CIDR{"1.1.1.1/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.workers[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.gcp.networks.nodes" ("10.250.0.0/16")`),
					}))
				})

				It("should forbid Internal CIDR to overlap with Node - and Worker CIDR", func() {
					overlappingCIDR := garden.CIDR("10.250.1.0/30")
					shoot.Spec.Cloud.GCP.Networks.Internal = &overlappingCIDR
					shoot.Spec.Cloud.GCP.Networks.Workers = []garden.CIDR{overlappingCIDR}
					shoot.Spec.Cloud.GCP.Networks.Nodes = &overlappingCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.internal"),
						"Detail": Equal(`must not be a subset of "spec.cloud.gcp.networks.nodes" ("10.250.1.0/30")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.internal"),
						"Detail": Equal(`must not be a subset of "spec.cloud.gcp.networks.workers[0]" ("10.250.1.0/30")`),
					}))
				})

				It("should invalid k8s networks", func() {
					shoot.Spec.Cloud.GCP.Networks.K8SNetworks = invalidK8sNetworks

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.nodes"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.pods"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.services"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})
			})

			It("should forbid non canonical CIDRs", func() {
				nodeCIDR := garden.CIDR("10.250.0.3/16")
				podCIDR := garden.CIDR("100.96.0.4/11")
				serviceCIDR := garden.CIDR("100.64.0.5/13")
				internal := garden.CIDR("10.10.0.4/24")
				shoot.Spec.Cloud.GCP.Networks.Internal = &internal
				shoot.Spec.Cloud.GCP.Networks.Workers = []garden.CIDR{"10.250.3.8/24"}
				shoot.Spec.Cloud.GCP.Networks.Nodes = &nodeCIDR
				shoot.Spec.Cloud.GCP.Networks.Services = &serviceCIDR
				shoot.Spec.Cloud.GCP.Networks.Pods = &podCIDR

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(5))

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.gcp.nodes"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.gcp.pods"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.gcp.services"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.gcp.networks.internal[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.gcp.networks.workers[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.GCPWorker{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
				}))
			})

			It("should enforce unique worker names", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.GCPWorker{
					{
						Worker:     worker,
						VolumeSize: "20Gi",
						VolumeType: "default",
					},
					{
						Worker:     worker,
						VolumeSize: "20Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[1]", fldPath)),
				}))
			})

			It("should forbid invalid worker configuration", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.GCPWorker{
					{
						Worker:     invalidWorker,
						VolumeSize: "hugo",
						VolumeType: "",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(7))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].machineType", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMin", fldPath)),
				}))
				Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMax", fldPath)),
				}))
				Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMax", fldPath)),
				}))
				Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeSize", fldPath)),
				}))
				Expect(*errorList[6]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeType", fldPath)),
				}))
			})

			It("should enforce workers min > 0 if max > 0", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.GCPWorker{
					{
						Worker:     workerAutoScalingInvalid,
						VolumeSize: "20Gi",
						VolumeType: "default",
					},
					{
						Worker:     worker,
						VolumeSize: "40Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMin", fldPath)),
				}))
			})

			It("should allow workers having min=max=0 if at least one pool is active", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.GCPWorker{
					{
						Worker:     worker,
						VolumeSize: "40Gi",
						VolumeType: "default",
					},
					{
						Worker:     workerAutoScalingMinMaxZero,
						VolumeSize: "20Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid worker pools with too less volume size", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.GCPWorker{
					{
						Worker:     worker,
						VolumeSize: "19Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeSize", fldPath)),
				}))
			})

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.GCP.Workers[0].Worker = invalidWorkerTooLongName

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid worker pools with names that are not DNS-1123 label compliant", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.GCPWorker{
					{
						Worker:     invalidWorkerName,
						VolumeSize: "20Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid an empty zones list", func() {
				shoot.Spec.Cloud.GCP.Zones = []string{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid updating networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.GCP.Networks.Workers[0] = garden.CIDR("10.250.0.0/24")
				newShoot.Spec.Cloud.GCP.Zones = []string{"another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})

			It("should forbid removing the GCP section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.GCP = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(HaveLen(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.cloud.aws/azure/gcp/alicloud/openstack/packet"),
				}))
			})

			Context("NodeCIDRMask validation", func() {
				var (
					defaultMaxPod           int32 = 110
					maxPod                  int32 = 260
					defaultNodeCIDRMaskSize       = 24
					testWorker              garden.Worker
				)

				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = &defaultNodeCIDRMaskSize
					shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &defaultMaxPod}
					testWorker = *worker.DeepCopy()
					testWorker.Name = "testworker"
				})

				It("should not return any errors", func() {
					worker.Kubelet = &garden.KubeletConfig{
						MaxPods: &defaultMaxPod,
					}
					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(0))
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							shoot.Spec.Cloud.GCP.Workers = append(shoot.Spec.Cloud.GCP.Workers, garden.GCPWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     testWorker,
							})

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))

							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
					Context("multiple worker pools", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							secondTestWorker := *testWorker.DeepCopy()
							secondTestWorker.Name = "testworker2"
							secondTestWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							shoot.Spec.Cloud.GCP.Workers = append(shoot.Spec.Cloud.GCP.Workers, garden.GCPWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     testWorker,
							}, garden.GCPWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     secondTestWorker,
							})

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})

					Context("Global default max pod", func() {
						It("should deny NodeCIDR with too few ips", func() {
							shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &maxPod}

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
				})
			})
		})

		Context("Alicloud specific validation", func() {
			var (
				fldPath  = "alicloud"
				alicloud *garden.Alicloud
			)

			BeforeEach(func() {
				alicloud = &garden.Alicloud{
					Networks: garden.AlicloudNetworks{
						K8SNetworks: k8sNetworks,
						VPC: garden.AlicloudVPC{
							CIDR: &vpcCIDR,
						},
						Workers: []garden.CIDR{"10.250.3.0/24"},
					},
					Workers: []garden.AlicloudWorker{
						{
							Worker:     worker,
							VolumeSize: "30Gi",
							VolumeType: "default",
						},
					},
					Zones: []string{"cn-beijing-f"},
				}

				shoot.Spec.Cloud.AWS = nil
				shoot.Spec.Cloud.Alicloud = alicloud
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			Context("CIDR", func() {

				It("should forbid invalid VPC CIDRs", func() {
					shoot.Spec.Cloud.Alicloud.Networks.VPC.CIDR = &invalidCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.vpc.cidr"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid invalid workers CIDR", func() {
					shoot.Spec.Cloud.Alicloud.Networks.Workers = []garden.CIDR{invalidCIDR}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.workers[0]"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid workers CIDR which are not in Nodes CIDR", func() {
					shoot.Spec.Cloud.Alicloud.Networks.Workers = []garden.CIDR{"1.1.1.1/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.workers[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.alicloud.networks.nodes" ("10.250.0.0/16")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.workers[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.alicloud.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid Node which are not in VPC CIDR", func() {
					notOverlappingCIDR := garden.CIDR("1.1.1.1/32")
					shoot.Spec.Cloud.Alicloud.Networks.K8SNetworks.Nodes = &notOverlappingCIDR
					shoot.Spec.Cloud.Alicloud.Networks.Workers = []garden.CIDR{notOverlappingCIDR}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.nodes"),
						"Detail": Equal(`must be a subset of "spec.cloud.alicloud.networks.vpc.cidr" ("10.0.0.0/8")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.workers[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.alicloud.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid Pod CIDR to overlap with VPC CIDR", func() {
					podCIDR := garden.CIDR("10.0.0.1/32")
					shoot.Spec.Cloud.Alicloud.Networks.K8SNetworks.Pods = &podCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.pods"),
						"Detail": Equal(`must not be a subset of "spec.cloud.alicloud.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid Services CIDR to overlap with VPC CIDR", func() {
					servicesCIDR := garden.CIDR("10.0.0.1/32")
					shoot.Spec.Cloud.Alicloud.Networks.K8SNetworks.Services = &servicesCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.services"),
						"Detail": Equal(`must not be a subset of "spec.cloud.alicloud.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should invalid k8s networks", func() {
					shoot.Spec.Cloud.Alicloud.Networks.K8SNetworks = invalidK8sNetworks

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.nodes"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.pods"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.services"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})
			})

			It("should forbid non canonical CIDRs", func() {
				vpcCIDR := garden.CIDR("10.0.0.3/8")
				nodeCIDR := garden.CIDR("10.250.0.3/16")
				podCIDR := garden.CIDR("100.96.0.4/11")
				serviceCIDR := garden.CIDR("100.64.0.5/13")

				shoot.Spec.Cloud.Alicloud.Networks.Workers = []garden.CIDR{"10.250.3.8/24"}
				shoot.Spec.Cloud.Alicloud.Networks.Nodes = &nodeCIDR
				shoot.Spec.Cloud.Alicloud.Networks.Services = &serviceCIDR
				shoot.Spec.Cloud.Alicloud.Networks.Pods = &podCIDR
				shoot.Spec.Cloud.Alicloud.Networks.VPC = garden.AlicloudVPC{CIDR: &vpcCIDR}

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(5))

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.alicloud.networks.vpc.cidr"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.alicloud.nodes"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.alicloud.pods"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.alicloud.services"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.alicloud.networks.workers[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.AlicloudWorker{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
				}))
			})

			It("should enforce unique worker names", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.AlicloudWorker{
					{
						Worker:     worker,
						VolumeSize: "30Gi",
						VolumeType: "default",
					},
					{
						Worker:     worker,
						VolumeSize: "30Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[1]", fldPath)),
				}))
			})

			It("should forbid invalid worker configuration", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.AlicloudWorker{
					{
						Worker:     invalidWorker,
						VolumeSize: "hugo",
						VolumeType: "",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(7))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].machineType", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMin", fldPath)),
				}))
				Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMax", fldPath)),
				}))
				Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMax", fldPath)),
				}))
				Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeSize", fldPath)),
				}))
				Expect(*errorList[6]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeType", fldPath)),
				}))
			})

			It("should enforce workers min > 0 if max > 0", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.AlicloudWorker{
					{
						Worker:     workerAutoScalingInvalid,
						VolumeSize: "30Gi",
						VolumeType: "default",
					},
					{
						Worker:     worker,
						VolumeSize: "40Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMin", fldPath)),
				}))
			})

			It("should allow workers having min=max=0 if at least one pool is active", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.AlicloudWorker{
					{
						Worker:     worker,
						VolumeSize: "40Gi",
						VolumeType: "default",
					},
					{
						Worker:     workerAutoScalingMinMaxZero,
						VolumeSize: "40Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid worker pools with too less volume size", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.AlicloudWorker{
					{
						Worker:     worker,
						VolumeSize: "10Gi",
						VolumeType: "gp2",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeSize", fldPath)),
				}))
			})

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.Alicloud.Workers[0].Worker = invalidWorkerTooLongName

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid worker pools with names that are not DNS-1123 label compliant", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.AlicloudWorker{
					{
						Worker:     invalidWorkerName,
						VolumeSize: "30Gi",
						VolumeType: "gp2",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid an empty zones list", func() {
				shoot.Spec.Cloud.Alicloud.Zones = []string{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid updating networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.Alicloud.Networks.Workers[0] = garden.CIDR("10.250.0.0/24")
				newShoot.Spec.Cloud.Alicloud.Zones = []string{"another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})

			It("should forbid removing the Alicloud section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.Alicloud = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(HaveLen(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.cloud.aws/azure/gcp/alicloud/openstack/packet"),
				}))
			})

			Context("NodeCIDRMask validation", func() {
				var (
					defaultMaxPod           int32 = 110
					maxPod                  int32 = 260
					defaultNodeCIDRMaskSize       = 24
					testWorker              garden.Worker
				)

				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = &defaultNodeCIDRMaskSize
					shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &defaultMaxPod}
					testWorker = *worker.DeepCopy()
					testWorker.Name = "testworker"
				})

				It("should not return any errors", func() {
					worker.Kubelet = &garden.KubeletConfig{
						MaxPods: &defaultMaxPod,
					}
					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(0))
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							shoot.Spec.Cloud.Alicloud.Workers = append(shoot.Spec.Cloud.Alicloud.Workers, garden.AlicloudWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     testWorker,
							})

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))

							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
					Context("multiple worker pools", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							secondTestWorker := *testWorker.DeepCopy()
							secondTestWorker.Name = "testworker2"
							secondTestWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							shoot.Spec.Cloud.Alicloud.Workers = append(shoot.Spec.Cloud.Alicloud.Workers, garden.AlicloudWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     testWorker,
							}, garden.AlicloudWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     secondTestWorker,
							})

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})

					Context("Global default max pod", func() {
						It("should deny NodeCIDR with too few ips", func() {
							shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &maxPod}

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
				})
			})
		})

		// BEGIN PACKET
		Context("Packet specific validation", func() {
			var (
				fldPath = "packet"
				packet  *garden.PacketCloud
			)

			BeforeEach(func() {
				packet = &garden.PacketCloud{
					Networks: garden.PacketNetworks{
						K8SNetworks: k8sNetworks,
					},
					Workers: []garden.PacketWorker{
						{
							Worker:     worker,
							VolumeSize: "20Gi",
							VolumeType: "default",
						},
					},
					Zones: []string{"EWR1"},
				}

				shoot.Spec.Cloud.AWS = nil
				shoot.Spec.Cloud.Packet = packet
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			Context("CIDR", func() {
				It("should forbid invalid k8s networks", func() {
					shoot.Spec.Cloud.Packet.Networks.K8SNetworks = invalidK8sNetworks

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.packet.networks.nodes"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.packet.networks.pods"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.packet.networks.services"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})
			})

			It("should forbid non canonical CIDRs", func() {
				podCIDR := garden.CIDR("100.96.0.4/11")
				serviceCIDR := garden.CIDR("100.64.0.5/13")

				shoot.Spec.Cloud.Packet.Networks.Services = &serviceCIDR
				shoot.Spec.Cloud.Packet.Networks.Pods = &podCIDR

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(2))

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.packet.pods"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.packet.services"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.Packet.Workers = []garden.PacketWorker{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
				}))))
			})

			It("should enforce unique worker names", func() {
				shoot.Spec.Cloud.Packet.Workers = []garden.PacketWorker{
					{
						Worker:     worker,
						VolumeSize: "20Gi",
						VolumeType: "default",
					},
					{
						Worker:     worker,
						VolumeSize: "20Gi",
						VolumeType: "default",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[1]", fldPath)),
				}))))
			})

			It("should forbid invalid worker configuration", func() {
				shoot.Spec.Cloud.Packet.Workers = []garden.PacketWorker{
					{
						Worker:     invalidWorker,
						VolumeSize: "hugo",
						VolumeType: "",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(7))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].machineType", fldPath)),
				}))
				Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeSize", fldPath)),
				}))
				Expect(*errorList[6]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeType", fldPath)),
				}))
			})

			It("should forbid worker pools with too less volume size", func() {
				shoot.Spec.Cloud.Packet.Workers = []garden.PacketWorker{
					{
						Worker:     worker,
						VolumeSize: "10Gi",
						VolumeType: "gp2",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeSize", fldPath)),
				}))))
			})

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.Packet.Workers[0].Worker = invalidWorkerTooLongName

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))))
			})

			It("should forbid worker pools with names that are not DNS-1123 label compliant", func() {
				shoot.Spec.Cloud.Packet.Workers = []garden.PacketWorker{
					{
						Worker:     invalidWorkerName,
						VolumeSize: "20Gi",
						VolumeType: "gp2",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))))
			})

			It("should forbid an empty zones list", func() {
				shoot.Spec.Cloud.Packet.Zones = []string{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))))
			})

			It("should forbid updating networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				cidr := garden.CIDR("10.250.0.0/24")
				newShoot.Spec.Cloud.Packet.Networks.Nodes = &cidr
				newShoot.Spec.Cloud.Packet.Zones = []string{"another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.nodes", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})

			It("should forbid removing the Packet section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.Packet = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.cloud.aws/azure/gcp/alicloud/openstack/packet"),
				}))))
			})

			Context("NodeCIDRMask validation", func() {
				var (
					defaultMaxPod           int32 = 110
					maxPod                  int32 = 260
					defaultNodeCIDRMaskSize       = 24
					testWorker              garden.Worker
				)

				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = &defaultNodeCIDRMaskSize
					shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &defaultMaxPod}
					testWorker = *worker.DeepCopy()
					testWorker.Name = "testworker"
				})

				It("should not return any errors", func() {
					worker.Kubelet = &garden.KubeletConfig{
						MaxPods: &defaultMaxPod,
					}
					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(0))
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							shoot.Spec.Cloud.Packet.Workers = append(shoot.Spec.Cloud.Packet.Workers, garden.PacketWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     testWorker,
							})

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))

							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
					Context("multiple worker pools", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							secondTestWorker := *testWorker.DeepCopy()
							secondTestWorker.Name = "testworker2"
							secondTestWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							shoot.Spec.Cloud.Packet.Workers = append(shoot.Spec.Cloud.Packet.Workers, garden.PacketWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     testWorker,
							}, garden.PacketWorker{
								VolumeSize: "35Gi",
								VolumeType: "default",
								Worker:     secondTestWorker,
							})

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})

					Context("Global default max pod", func() {
						It("should deny NodeCIDR with too few ips", func() {
							shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &maxPod}

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
				})
			})
		})
		// END PACKET

		Context("OpenStack specific validation", func() {
			var (
				fldPath        = "openstack"
				openStackCloud *garden.OpenStackCloud
			)

			BeforeEach(func() {
				openStackCloud = &garden.OpenStackCloud{
					FloatingPoolName:     "my-pool",
					LoadBalancerProvider: "haproxy",
					Networks: garden.OpenStackNetworks{
						K8SNetworks: k8sNetworks,
						Workers:     []garden.CIDR{"10.250.0.0/16"},
						Router: &garden.OpenStackRouter{
							ID: "router1234",
						},
					},
					Workers: []garden.OpenStackWorker{
						{
							Worker: worker,
						},
					},
					Zones: []string{"europe-1a"},
				}
				shoot.Spec.Cloud.AWS = nil
				shoot.Spec.Cloud.OpenStack = openStackCloud
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid invalid floating pool name configuration", func() {
				shoot.Spec.Cloud.OpenStack.FloatingPoolName = ""

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.floatingPoolName", fldPath)),
				}))
			})

			It("should forbid invalid load balancer provider configuration", func() {
				shoot.Spec.Cloud.OpenStack.LoadBalancerProvider = ""

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.loadBalancerProvider", fldPath)),
				}))
			})

			Context("CIDR", func() {
				It("should forbid more than one CIDR", func() {
					shoot.Spec.Cloud.OpenStack.Networks.Workers = []garden.CIDR{"10.250.0.1/32", "10.250.0.2/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.openstack.networks.workers"),
						"Detail": Equal("must specify only one worker cidr"),
					}))
				})

				It("should forbid invalid workers CIDR", func() {
					shoot.Spec.Cloud.OpenStack.Networks.Workers = []garden.CIDR{invalidCIDR}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.openstack.networks.workers[0]"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid workers CIDR which are not in Nodes CIDR", func() {
					shoot.Spec.Cloud.OpenStack.Networks.Workers = []garden.CIDR{"1.1.1.1/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.openstack.networks.workers[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.openstack.networks.nodes" ("10.250.0.0/16")`),
					}))
				})

				It("should invalid k8s networks", func() {
					shoot.Spec.Cloud.OpenStack.Networks.K8SNetworks = invalidK8sNetworks

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.openstack.networks.nodes"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.openstack.networks.pods"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.openstack.networks.services"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})
			})

			It("should forbid non canonical CIDRs", func() {
				nodeCIDR := garden.CIDR("10.250.0.3/16")
				podCIDR := garden.CIDR("100.96.0.4/11")
				serviceCIDR := garden.CIDR("100.64.0.5/13")

				shoot.Spec.Cloud.OpenStack.Networks.Workers = []garden.CIDR{"10.250.3.8/24"}
				shoot.Spec.Cloud.OpenStack.Networks.Nodes = &nodeCIDR
				shoot.Spec.Cloud.OpenStack.Networks.Services = &serviceCIDR
				shoot.Spec.Cloud.OpenStack.Networks.Pods = &podCIDR

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(4))

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.openstack.nodes"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.openstack.pods"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.openstack.services"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.openstack.networks.workers[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.OpenStack.Workers = []garden.OpenStackWorker{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
				}))
			})

			It("should enforce unique worker names", func() {
				shoot.Spec.Cloud.OpenStack.Workers = []garden.OpenStackWorker{
					{
						Worker: worker,
					},
					{
						Worker: worker,
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[1]", fldPath)),
				}))
			})

			It("should forbid invalid worker configuration", func() {
				shoot.Spec.Cloud.OpenStack.Workers = []garden.OpenStackWorker{
					{
						Worker: invalidWorker,
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(5))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].machineType", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMin", fldPath)),
				}))
				Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMax", fldPath)),
				}))
				Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMax", fldPath)),
				}))
			})

			It("should enforce workers min > 0 if max > 0", func() {
				shoot.Spec.Cloud.OpenStack.Workers = []garden.OpenStackWorker{
					{
						Worker: workerAutoScalingInvalid,
					},
					{
						Worker: worker,
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMin", fldPath)),
				}))
			})

			It("should allow workers having min=max=0 if at least one pool is active", func() {
				shoot.Spec.Cloud.OpenStack.Workers = []garden.OpenStackWorker{
					{
						Worker: workerAutoScalingMinMaxZero,
					},
					{
						Worker: worker,
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.OpenStack.Workers = []garden.OpenStackWorker{
					{
						Worker: invalidWorkerTooLongName,
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid worker pools with names that are not DNS-1123 label compliant", func() {
				shoot.Spec.Cloud.OpenStack.Workers = []garden.OpenStackWorker{
					{
						Worker: invalidWorkerName,
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid an empty zones list", func() {
				shoot.Spec.Cloud.OpenStack.Zones = []string{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid updating networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.OpenStack.Networks.Workers[0] = garden.CIDR("10.250.0.0/24")
				newShoot.Spec.Cloud.OpenStack.Zones = []string{"another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})

			It("should forbid removing the OpenStack section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.OpenStack = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(HaveLen(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.cloud.aws/azure/gcp/alicloud/openstack/packet"),
				}))
			})

			Context("NodeCIDRMask validation", func() {
				var (
					defaultMaxPod           int32 = 110
					maxPod                  int32 = 260
					defaultNodeCIDRMaskSize       = 24
					testWorker              garden.Worker
				)

				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = &defaultNodeCIDRMaskSize
					shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &defaultMaxPod}
					testWorker = *worker.DeepCopy()
					testWorker.Name = "testworker"
				})

				It("should not return any errors", func() {
					worker.Kubelet = &garden.KubeletConfig{
						MaxPods: &defaultMaxPod,
					}
					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(0))
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							shoot.Spec.Cloud.OpenStack.Workers = append(shoot.Spec.Cloud.OpenStack.Workers, garden.OpenStackWorker{
								Worker: testWorker,
							})

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))

							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
					Context("multiple worker pools", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							secondTestWorker := *testWorker.DeepCopy()
							secondTestWorker.Name = "testworker2"
							secondTestWorker.Kubelet = &garden.KubeletConfig{
								MaxPods: &maxPod,
							}

							shoot.Spec.Cloud.OpenStack.Workers = append(shoot.Spec.Cloud.OpenStack.Workers, garden.OpenStackWorker{
								Worker: testWorker,
							}, garden.OpenStackWorker{
								Worker: secondTestWorker,
							})

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})

					Context("Global default max pod", func() {
						It("should deny NodeCIDR with too few ips", func() {
							shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &maxPod}

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
				})
			})
		})

		Context("dns section", func() {
			It("should forbid specifying a provider without a domain", func() {
				shoot.Spec.DNS.Domain = makeStringPointer("foo/bar.baz")
				shoot.Spec.DNS.Provider = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.domain"),
				}))))
			})

			It("should allow specifying the 'unmanaged' provider without a domain", func() {
				shoot.Spec.DNS.Domain = makeStringPointer(garden.DNSUnmanaged)
				shoot.Spec.DNS.Provider = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid specifying invalid domain", func() {
				shoot.Spec.DNS.Provider = nil
				shoot.Spec.DNS.Domain = makeStringPointer("foo/bar.baz")

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.domain"),
				}))))
			})

			It("should forbid specifying a secret name when provider equals 'unmanaged'", func() {
				provider := garden.DNSUnmanaged
				shoot.Spec.DNS.Provider = &provider
				shoot.Spec.DNS.SecretName = makeStringPointer("")

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.secretName"),
				}))))
			})

			It("should require a provider if a secret name is given", func() {
				shoot.Spec.DNS.Provider = nil
				shoot.Spec.DNS.SecretName = makeStringPointer("")

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.dns.provider"),
				}))))
			})

			It("should forbid updating the dns domain", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.DNS.Domain = makeStringPointer("another-domain.com")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.domain"),
				}))))
			})

			It("should forbid updating the dns provider", func() {
				newShoot := prepareShootForUpdate(shoot)
				provider := "some-other-provider"
				newShoot.Spec.DNS.Provider = &provider

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.provider"),
				}))))
			})

			It("should allow updating the dns secret name", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.DNS.SecretName = makeStringPointer("my-dns-secret")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(HaveLen(0))
			})
		})

		Context("OIDC validation", func() {
			It("should forbid unsupported OIDC configuration", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.CABundle = makeStringPointer("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = makeStringPointer("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.GroupsClaim = makeStringPointer("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.GroupsPrefix = makeStringPointer("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.IssuerURL = makeStringPointer("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.UsernameClaim = makeStringPointer("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.UsernamePrefix = makeStringPointer("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.RequiredClaims = map[string]string{}
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.SigningAlgs = []string{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.caBundle"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.clientID"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.groupsClaim"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.groupsPrefix"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.issuerURL"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.signingAlgs"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.usernameClaim"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.usernamePrefix"),
				}))))
			})

			It("should forbid unsupported OIDC configuration (for K8S >= v1.10)", func() {
				shoot.Spec.Kubernetes.Version = "1.10.1"
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.RequiredClaims = map[string]string{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.requiredClaims"),
				}))
			})

			It("should allow supported OIDC configuration (for K8S >= v1.11)", func() {
				shoot.Spec.Kubernetes.Version = "1.11.1"
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.RequiredClaims = map[string]string{
					"some": "claim",
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})
		})

		Context("admission plugin validation", func() {
			It("should allow not specifying admission plugins", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid specifying admission plugins without a name", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []garden.AdmissionPlugin{
					{
						Name: "",
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.kubernetes.kubeAPIServer.admissionPlugins[0].name"),
				}))
			})
		})

		Context("KubeControllerManager validation < 1.12", func() {
			It("should forbid unsupported HPA configuration", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.SyncPeriod = makeDurationPointer(100 * time.Millisecond)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.Tolerance = makeFloat64Pointer(0)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleDelay = makeDurationPointer(-1 * time.Second)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.UpscaleDelay = makeDurationPointer(-1 * time.Second)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.syncPeriod"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.tolerance"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.downscaleDelay"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.upscaleDelay"),
				}))))
			})

			It("should forbid unsupported HPA field configuration for versions < 1.12", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleStabilization = makeDurationPointer(5 * time.Minute)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.InitialReadinessDelay = makeDurationPointer(1 * time.Second)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.CPUInitializationPeriod = makeDurationPointer(5 * time.Minute)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.downscaleStabilization"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.initialReadinessDelay"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.cpuInitializationPeriod"),
				}))))
			})
		})

		Context("KubeControllerManager validation in versions > 1.12", func() {
			BeforeEach(func() {
				shoot.Spec.Kubernetes.Version = "1.12.1"
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleDelay = nil
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.UpscaleDelay = nil
			})

			It("should forbid unsupported HPA configuration in versions > 1.12", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleStabilization = makeDurationPointer(-1 * time.Second)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.InitialReadinessDelay = makeDurationPointer(-1 * time.Second)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.CPUInitializationPeriod = makeDurationPointer(-1 * time.Second)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.downscaleStabilization"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.initialReadinessDelay"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.cpuInitializationPeriod"),
				}))))
			})

			It("should fail when using configuration parameters from versions older than 1.12", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.UpscaleDelay = makeDurationPointer(1 * time.Minute)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleDelay = makeDurationPointer(1 * time.Second)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.upscaleDelay"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.downscaleDelay"),
				}))))
			})

			It("should succeed when using valid v1.12 configuration parameters", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleStabilization = makeDurationPointer(5 * time.Minute)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.InitialReadinessDelay = makeDurationPointer(30 * time.Second)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.CPUInitializationPeriod = makeDurationPointer(5 * time.Minute)

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(0))
			})
		})

		Context("KubeControllerManager configuration validation", func() {
			It("should fail updating immutable fields", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeIntPointer(24)

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeIntPointer(22)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
					"Detail": ContainSubstring(`field is immutable`),
				}))
			})

			It("should succeed not changing immutable fields", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeIntPointer(24)

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeIntPointer(24)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should fail when nodeCIDRMaskSize is out of upper boundary", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeIntPointer(32)

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
				})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
					}))))
			})

			It("should fail when nodeCIDRMaskSize is out of lower boundary", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeIntPointer(0)

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
				})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
					}))))
			})

			It("should succeed when nodeCIDRMaskSize is within boundaries", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeIntPointer(22)

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(BeEmpty())
			})
		})

		Context("KubeProxy validation", func() {
			BeforeEach(func() {
				shoot.Spec.Kubernetes.KubeProxy = &garden.KubeProxyConfig{}
			})

			It("should succeed when using IPTables mode", func() {
				mode := garden.ProxyModeIPTables
				shoot.Spec.Kubernetes.KubeProxy.Mode = &mode
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())

			})

			It("should succeed when using IPVS mode", func() {
				mode := garden.ProxyModeIPVS
				shoot.Spec.Kubernetes.KubeProxy.Mode = &mode
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())

			})

			It("should fail when using nil proxy mode", func() {
				shoot.Spec.Kubernetes.KubeProxy.Mode = nil
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.kubernetes.kubeProxy.mode"),
				}))))
			})

			It("should fail when using unknown proxy mode", func() {
				m := garden.ProxyMode("fooMode")
				shoot.Spec.Kubernetes.KubeProxy.Mode = &m
				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.kubernetes.kubeProxy.mode"),
				}))))
			})

			It("should fail when using kuberntes version 1.14.2 and proxy mode is changed", func() {
				mode := garden.ProxyMode("IPVS")
				kubernetesConfig := garden.KubernetesConfig{nil}
				config := garden.KubeProxyConfig{
					KubernetesConfig: kubernetesConfig,
					Mode:             &mode,
				}
				shoot.Spec.Kubernetes.KubeProxy = &config
				shoot.Spec.Kubernetes.Version = "1.14.2"
				oldMode := garden.ProxyMode("IPTables")
				oldConfig := garden.KubeProxyConfig{
					KubernetesConfig: kubernetesConfig,
					Mode:             &oldMode,
				}
				shoot.Spec.Kubernetes.KubeProxy.Mode = &mode
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.Kubernetes.KubeProxy = &oldConfig
				errorList := ValidateShootSpecUpdate(&shoot.Spec, &oldShoot.Spec, shoot.DeletionTimestamp != nil, field.NewPath("spec"))
				Expect(errorList).ToNot(BeEmpty())
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.kubernetes.kubeProxy.mode"),
					"Detail": Equal(`field is immutable`),
				}))
			})

			It("should be successful when using kuberntes version 1.14.1 and proxy mode stays the same", func() {
				mode := garden.ProxyMode("IPVS")
				shoot.Spec.Kubernetes.Version = "1.14.1"
				shoot.Spec.Kubernetes.KubeProxy.Mode = &mode
				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(2))
			})
		})

		Context("ClusterAutoscaler validation", func() {
			DescribeTable("cluster autoscaler values",
				func(clusterAutoscaler garden.ClusterAutoscaler, matcher gomegatypes.GomegaMatcher) {
					Expect(ValidateClusterAutoscaler(clusterAutoscaler, nil)).To(matcher)
				},
				Entry("valid", garden.ClusterAutoscaler{}, BeEmpty()),
				Entry("valid with threshold", garden.ClusterAutoscaler{
					ScaleDownUtilizationThreshold: makeFloat64Pointer(0.5),
				}, BeEmpty()),
				Entry("invalid negative threshold", garden.ClusterAutoscaler{
					ScaleDownUtilizationThreshold: makeFloat64Pointer(-0.5),
				}, ConsistOf(field.Invalid(field.NewPath("scaleDownUtilizationThreshold"), -0.5, "can not be negative"))),
				Entry("invalid > 1 threshold", garden.ClusterAutoscaler{
					ScaleDownUtilizationThreshold: makeFloat64Pointer(1.5),
				}, ConsistOf(field.Invalid(field.NewPath("scaleDownUtilizationThreshold"), 1.5, "can not be greater than 1.0"))),
			)
		})

		Context("AuditConfig validation", func() {
			It("should forbid empty name", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name = ""
				errorList := ValidateShoot(shoot)

				Expect(errorList).ToNot(BeEmpty())
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.kubernetes.kubeAPIServer.auditConfig.auditPolicy.configMapRef.name"),
				}))))
			})

			It("should allow nil AuditConfig", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig = nil
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})
		})

		It("should require a kubernetes version", func() {
			shoot.Spec.Kubernetes.Version = ""

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.kubernetes.version"),
			}))
		})
		It("should forbid removing the kubernetes version", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.Kubernetes.Version = ""

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(HaveLen(2))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.kubernetes.version"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.kubernetes.version"),
			}))
		})

		It("should forbid kubernetes version downgrades", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.Kubernetes.Version = "1.7.2"

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.kubernetes.version"),
			}))
		})

		It("should forbid kubernetes version upgrades skipping a minor version", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.Kubernetes.Version = "1.10.1"

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.kubernetes.version"),
			}))
		})

		Context("networking section", func() {
			It("should forbid not specifying the networking section", func() {
				shoot.Spec.Networking = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.networking"),
				}))))
			})

			It("should forbid not specifying a networking type", func() {
				shoot.Spec.Networking.Type = ""

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.networking.type"),
				}))))
			})
		})

		Context("maintenance section", func() {
			It("should forbid not specifying the maintenance section", func() {
				shoot.Spec.Maintenance = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.maintenance"),
				}))
			})

			It("should forbid not specifying the auto update section", func() {
				shoot.Spec.Maintenance.AutoUpdate = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.maintenance.autoUpdate"),
				}))
			})

			It("should forbid not specifying the time window section", func() {
				shoot.Spec.Maintenance.TimeWindow = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.maintenance.timeWindow"),
				}))
			})

			It("should forbid invalid formats for the time window begin and end values", func() {
				shoot.Spec.Maintenance.TimeWindow.Begin = "invalidformat"
				shoot.Spec.Maintenance.TimeWindow.End = "invalidformat"

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.maintenance.timeWindow.begin/end"),
				}))))
			})

			It("should forbid time windows greater than 6 hours", func() {
				shoot.Spec.Maintenance.TimeWindow.Begin = "145000+0100"
				shoot.Spec.Maintenance.TimeWindow.End = "215000+0100"

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.maintenance.timeWindow"),
				}))))
			})

			It("should forbid time windows smaller than 30 minutes", func() {
				shoot.Spec.Maintenance.TimeWindow.Begin = "225000+0100"
				shoot.Spec.Maintenance.TimeWindow.End = "231000+0100"

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.maintenance.timeWindow"),
				}))))
			})

			It("should allow time windows which overlap over two days", func() {
				shoot.Spec.Maintenance.TimeWindow.Begin = "230000+0100"
				shoot.Spec.Maintenance.TimeWindow.End = "010000+0100"

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})
		})

		It("should forbid updating the spec for shoots with deletion timestamp", func() {
			newShoot := prepareShootForUpdate(shoot)
			deletionTimestamp := metav1.NewTime(time.Now())
			shoot.ObjectMeta.DeletionTimestamp = &deletionTimestamp
			newShoot.ObjectMeta.DeletionTimestamp = &deletionTimestamp
			newShoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec"),
			}))
		})

		It("should allow updating the metadata for shoots with deletion timestamp", func() {
			newShoot := prepareShootForUpdate(shoot)
			deletionTimestamp := metav1.NewTime(time.Now())
			shoot.ObjectMeta.DeletionTimestamp = &deletionTimestamp
			newShoot.ObjectMeta.DeletionTimestamp = &deletionTimestamp
			newShoot.ObjectMeta.Labels = map[string]string{
				"new-key": "new-value",
			}

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(HaveLen(0))
		})
	})

	Describe("#ValidateShootStatus, #ValidateShootStatusUpdate", func() {
		var shoot *garden.Shoot

		BeforeEach(func() {
			shoot = &garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "my-namespace",
				},
				Spec:   garden.ShootSpec{},
				Status: garden.ShootStatus{},
			}
		})

		Context("uid checks", func() {
			It("should allow setting the uid", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Status.UID = types.UID("1234")

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid changing the uid", func() {
				newShoot := prepareShootForUpdate(shoot)
				shoot.Status.UID = types.UID("1234")
				newShoot.Status.UID = types.UID("1235")

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("status.uid"),
				}))
			})
		})

		Context("technical id checks", func() {
			It("should allow setting the technical id", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Status.TechnicalID = "shoot--foo--bar"

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid changing the technical id", func() {
				newShoot := prepareShootForUpdate(shoot)
				shoot.Status.TechnicalID = "shoot-foo-bar"
				newShoot.Status.TechnicalID = "shoot--foo--bar"

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("status.technicalID"),
				}))
			})
		})
	})

	Describe("#ValidateBackupInfrastructure", func() {
		var backupInfrastructure *garden.BackupInfrastructure

		BeforeEach(func() {
			backupInfrastructure = &garden.BackupInfrastructure{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-backupinfrastructure",
					Namespace: "garden",
					Annotations: map[string]string{
						common.BackupInfrastructureForceDeletion: "true",
					},
				},
				Spec: garden.BackupInfrastructureSpec{
					Seed:     "aws",
					ShootUID: types.UID(utils.ComputeSHA1Hex([]byte("shoot-garden-backup-infrastructure"))),
				},
			}
		})

		It("should not return any errors", func() {
			errorList := ValidateBackupInfrastructure(backupInfrastructure)

			Expect(errorList).To(HaveLen(0))
		})

		It("should forbid BackupInfrastructure resources with empty metadata", func() {
			backupInfrastructure.ObjectMeta = metav1.ObjectMeta{}

			errorList := ValidateBackupInfrastructure(backupInfrastructure)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.namespace"),
				}))))
		})

		It("should forbid BackupInfrastructure specification with empty or invalid keys", func() {
			backupInfrastructure.Spec.Seed = ""
			backupInfrastructure.Spec.ShootUID = ""

			errorList := ValidateBackupInfrastructure(backupInfrastructure)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.seed"),
			})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.shootUID"),
				}))))
		})

		It("should forbid updating some keys", func() {
			newBackupInfrastructure := prepareBackupInfrastructureForUpdate(backupInfrastructure)
			newBackupInfrastructure.Spec.Seed = "another-seed"
			newBackupInfrastructure.Spec.ShootUID = "another-uid"

			errorList := ValidateBackupInfrastructureUpdate(newBackupInfrastructure, backupInfrastructure)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.seed"),
			})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.shootUID"),
				}))))
		})
	})
})

// Helper functions

func makeStringPointer(s string) *string {
	ptr := s
	return &ptr
}

func makeDurationPointer(d time.Duration) *metav1.Duration {
	return &metav1.Duration{Duration: d}
}

func makeFloat64Pointer(f float64) *float64 {
	ptr := f
	return &ptr
}

func makeIntPointer(i int) *int {
	ptr := i
	return &ptr
}

func makeBoolPointer(i bool) *bool {
	ptr := i
	return &ptr
}

func prepareShootForUpdate(shoot *garden.Shoot) *garden.Shoot {
	s := shoot.DeepCopy()
	s.ResourceVersion = "1"
	return s
}

func prepareSeedForUpdate(seed *garden.Seed) *garden.Seed {
	s := seed.DeepCopy()
	s.ResourceVersion = "1"
	return s
}

func prepareBackupInfrastructureForUpdate(backupInfrastructure *garden.BackupInfrastructure) *garden.BackupInfrastructure {
	b := backupInfrastructure.DeepCopy()
	b.ResourceVersion = "1"
	return b
}

func prepareSecretBindingForUpdate(secretBinding *garden.SecretBinding) *garden.SecretBinding {
	s := secretBinding.DeepCopy()
	s.ResourceVersion = "1"
	return s
}

func prepareProjectForUpdate(project *garden.Project) *garden.Project {
	p := project.DeepCopy()
	p.ResourceVersion = "1"
	return p
}
