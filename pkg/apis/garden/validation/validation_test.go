// Copyright 2018 The Gardener Authors.
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
	. "github.com/gardener/gardener/pkg/apis/garden/validation"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("validation", func() {
	Describe("#ValidateCloudProfile", func() {
		var (
			metadata = metav1.ObjectMeta{
				Name: "profile",
			}
			dnsProviderConstraint = []garden.DNSProviderConstraint{
				{
					Name: garden.DNSUnmanaged,
				},
			}
			kubernetesVersionConstraint = garden.KubernetesConstraints{
				Versions: []string{"1.6.4"},
			}
			machineTypesConstraint = []garden.MachineType{
				{
					Name:   "machine-type-1",
					CPUs:   2,
					GPUs:   0,
					Memory: resource.Quantity{Format: "100Gi"},
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

			invalidDNSProviders = []garden.DNSProviderConstraint{
				{
					Name: garden.DNSProvider("some-unsupported-provider"),
				},
			}
			invalidKubernetes   = []string{"1.6"}
			invalidMachineTypes = []garden.MachineType{
				{
					Name:   "",
					CPUs:   -1,
					GPUs:   -1,
					Memory: resource.Quantity{Format: "100Gi"},
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

			Expect(len(errorList)).To(Equal(2))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.aws/azure/gcp/openstack"),
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
								DNSProviders: dnsProviderConstraint,
								Kubernetes:   kubernetesVersionConstraint,
								MachineTypes: machineTypesConstraint,
								VolumeTypes:  volumeTypesConstraint,
								Zones:        zonesConstraint,
							},
							MachineImages: []garden.AWSMachineImage{
								{
									Region: "eu-west-1",
									AMI:    "ami-12345678",
								},
							},
						},
					},
				}
			})

			It("should not return any errors", func() {
				errorList := ValidateCloudProfile(awsCloudProfile)

				Expect(len(errorList)).To(Equal(0))
			})

			Context("dns provider constraints", func() {
				It("should enforce that at least one provider has been defined", func() {
					awsCloudProfile.Spec.AWS.Constraints.DNSProviders = []garden.DNSProviderConstraint{}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.dnsProviders", fldPath)),
					}))
				})

				It("should forbid unsupported providers", func() {
					awsCloudProfile.Spec.AWS.Constraints.DNSProviders = invalidDNSProviders

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.dnsProviders[0]", fldPath)),
					}))
				})
			})

			Context("kubernetes version constraints", func() {
				It("should enforce that at least one version has been defined", func() {
					awsCloudProfile.Spec.AWS.Constraints.Kubernetes.Versions = []string{}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.versions", fldPath)),
					}))
				})

				It("should forbid versions of a not allowed pattern", func() {
					awsCloudProfile.Spec.AWS.Constraints.Kubernetes.Versions = invalidKubernetes

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.versions[0]", fldPath)),
					}))
				})
			})

			Context("machine types validation", func() {
				It("should enforce that at least one machine type has been defined", func() {
					awsCloudProfile.Spec.AWS.Constraints.MachineTypes = []garden.MachineType{}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes", fldPath)),
					}))
				})

				It("should forbid machine types with unsupported property values", func() {
					awsCloudProfile.Spec.AWS.Constraints.MachineTypes = invalidMachineTypes

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(len(errorList)).To(Equal(3))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].name", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].cpus", fldPath)),
					}))
					Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].gpus", fldPath)),
					}))
				})
			})

			Context("volume types validation", func() {
				It("should enforce that at least one volume type has been defined", func() {
					awsCloudProfile.Spec.AWS.Constraints.VolumeTypes = []garden.VolumeType{}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes", fldPath)),
					}))
				})

				It("should forbid volume types with unsupported property values", func() {
					awsCloudProfile.Spec.AWS.Constraints.VolumeTypes = invalidVolumeTypes

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(len(errorList)).To(Equal(2))
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

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones", fldPath)),
					}))
				})

				It("should forbid zones with unsupported name values", func() {
					awsCloudProfile.Spec.AWS.Constraints.Zones = invalidZones

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(len(errorList)).To(Equal(2))
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

			Context("machine image validation", func() {
				It("should enforce that at least one machine image has been defined", func() {
					awsCloudProfile.Spec.AWS.MachineImages = []garden.AWSMachineImage{}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.machineImages", fldPath)),
					}))
				})

				It("should forbid machine images with unsupported format", func() {
					awsCloudProfile.Spec.AWS.MachineImages = []garden.AWSMachineImage{
						{
							Region: "",
							AMI:    "ami-of-supported-format",
						},
					}

					errorList := ValidateCloudProfile(awsCloudProfile)

					Expect(len(errorList)).To(Equal(2))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.machineImages[0].region", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.machineImages[0].ami", fldPath)),
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
								DNSProviders: dnsProviderConstraint,
								Kubernetes:   kubernetesVersionConstraint,
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
							MachineImage: garden.AzureMachineImage{
								Channel: "Beta",
								Version: "coreos-1.6.4",
							},
						},
					},
				}
			})

			Context("dns provider constraints", func() {
				It("should enforce that at least one provider has been defined", func() {
					azureCloudProfile.Spec.Azure.Constraints.DNSProviders = []garden.DNSProviderConstraint{}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.dnsProviders", fldPath)),
					}))
				})

				It("should forbid unsupported providers", func() {
					azureCloudProfile.Spec.Azure.Constraints.DNSProviders = invalidDNSProviders

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.dnsProviders[0]", fldPath)),
					}))
				})
			})

			Context("kubernetes version constraints", func() {
				It("should enforce that at least one version has been defined", func() {
					azureCloudProfile.Spec.Azure.Constraints.Kubernetes.Versions = []string{}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.versions", fldPath)),
					}))
				})

				It("should forbid versions of a not allowed pattern", func() {
					azureCloudProfile.Spec.Azure.Constraints.Kubernetes.Versions = invalidKubernetes

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.versions[0]", fldPath)),
					}))
				})
			})

			Context("machine types validation", func() {
				It("should enforce that at least one machine type has been defined", func() {
					azureCloudProfile.Spec.Azure.Constraints.MachineTypes = []garden.MachineType{}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes", fldPath)),
					}))
				})

				It("should forbid machine types with unsupported property values", func() {
					azureCloudProfile.Spec.Azure.Constraints.MachineTypes = invalidMachineTypes

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(len(errorList)).To(Equal(3))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].name", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].cpus", fldPath)),
					}))
					Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].gpus", fldPath)),
					}))
				})
			})

			Context("volume types validation", func() {
				It("should enforce that at least one volume type has been defined", func() {
					azureCloudProfile.Spec.Azure.Constraints.VolumeTypes = []garden.VolumeType{}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes", fldPath)),
					}))
				})

				It("should forbid volume types with unsupported property values", func() {
					azureCloudProfile.Spec.Azure.Constraints.VolumeTypes = invalidVolumeTypes

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(len(errorList)).To(Equal(2))
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

					Expect(len(errorList)).To(Equal(1))
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

					Expect(len(errorList)).To(Equal(2))
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

					Expect(len(errorList)).To(Equal(1))
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

					Expect(len(errorList)).To(Equal(2))
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

			Context("machine image validation", func() {
				It("should forbid machine images with unsupported format", func() {
					azureCloudProfile.Spec.Azure.MachineImage = garden.AzureMachineImage{
						Channel: "",
						Version: "",
					}

					errorList := ValidateCloudProfile(azureCloudProfile)

					Expect(len(errorList)).To(Equal(2))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.machineImage.channel", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.machineImage.version", fldPath)),
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
								DNSProviders: dnsProviderConstraint,
								Kubernetes:   kubernetesVersionConstraint,
								MachineTypes: machineTypesConstraint,
								VolumeTypes:  volumeTypesConstraint,
								Zones:        zonesConstraint,
							},
							MachineImage: garden.GCPMachineImage{
								Name: "coreos-1.6.4",
							},
						},
					},
				}
			})

			Context("dns provider constraints", func() {
				It("should enforce that at least one provider has been defined", func() {
					gcpCloudProfile.Spec.GCP.Constraints.DNSProviders = []garden.DNSProviderConstraint{}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.dnsProviders", fldPath)),
					}))
				})

				It("should forbid unsupported providers", func() {
					gcpCloudProfile.Spec.GCP.Constraints.DNSProviders = invalidDNSProviders

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.dnsProviders[0]", fldPath)),
					}))
				})
			})

			Context("kubernetes version constraints", func() {
				It("should enforce that at least one version has been defined", func() {
					gcpCloudProfile.Spec.GCP.Constraints.Kubernetes.Versions = []string{}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.versions", fldPath)),
					}))
				})

				It("should forbid versions of a not allowed pattern", func() {
					gcpCloudProfile.Spec.GCP.Constraints.Kubernetes.Versions = invalidKubernetes

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.versions[0]", fldPath)),
					}))
				})
			})

			Context("machine types validation", func() {
				It("should enforce that at least one machine type has been defined", func() {
					gcpCloudProfile.Spec.GCP.Constraints.MachineTypes = []garden.MachineType{}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes", fldPath)),
					}))
				})

				It("should forbid machine types with unsupported property values", func() {
					gcpCloudProfile.Spec.GCP.Constraints.MachineTypes = invalidMachineTypes

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(len(errorList)).To(Equal(3))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].name", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].cpus", fldPath)),
					}))
					Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].gpus", fldPath)),
					}))
				})
			})

			Context("volume types validation", func() {
				It("should enforce that at least one volume type has been defined", func() {
					gcpCloudProfile.Spec.GCP.Constraints.VolumeTypes = []garden.VolumeType{}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.volumeTypes", fldPath)),
					}))
				})

				It("should forbid volume types with unsupported property values", func() {
					gcpCloudProfile.Spec.GCP.Constraints.VolumeTypes = invalidVolumeTypes

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(len(errorList)).To(Equal(2))
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

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones", fldPath)),
					}))
				})

				It("should forbid zones with unsupported name values", func() {
					gcpCloudProfile.Spec.GCP.Constraints.Zones = invalidZones

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(len(errorList)).To(Equal(2))
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

			Context("machine image validation", func() {
				It("should forbid machine images with unsupported format", func() {
					gcpCloudProfile.Spec.GCP.MachineImage = garden.GCPMachineImage{
						Name: "",
					}

					errorList := ValidateCloudProfile(gcpCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.machineImage.name", fldPath)),
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
								DNSProviders: dnsProviderConstraint,
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
								MachineTypes: machineTypesConstraint,
								Zones:        zonesConstraint,
							},
							KeyStoneURL: "http://url-to-keystone/v3",
							CABundle:    "-----BEGIN CERTIFICATE-----\nMIICRzCCAfGgAwIBAgIJALMb7ecMIk3MMA0GCSqGSIb3DQEBCwUAMH4xCzAJBgNV\nBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNVBAcMBkxvbmRvbjEYMBYGA1UE\nCgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1JVCBEZXBhcnRtZW50MRswGQYD\nVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTAwIBcNMTcwNDI2MjMyNjUyWhgPMjExNzA0\nMDIyMzI2NTJaMH4xCzAJBgNVBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNV\nBAcMBkxvbmRvbjEYMBYGA1UECgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1J\nVCBEZXBhcnRtZW50MRswGQYDVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTAwXDANBgkq\nhkiG9w0BAQEFAANLADBIAkEAtBMa7NWpv3BVlKTCPGO/LEsguKqWHBtKzweMY2CV\ntAL1rQm913huhxF9w+ai76KQ3MHK5IVnLJjYYA5MzP2H5QIDAQABo1AwTjAdBgNV\nHQ4EFgQU22iy8aWkNSxv0nBxFxerfsvnZVMwHwYDVR0jBBgwFoAU22iy8aWkNSxv\n0nBxFxerfsvnZVMwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAANBAEOefGbV\nNcHxklaW06w6OBYJPwpIhCVozC1qdxGX1dg8VkEKzjOzjgqVD30m59OFmSlBmHsl\nnkVA6wyOSDYBf3o=\n-----END CERTIFICATE-----",
							MachineImage: garden.OpenStackMachineImage{
								Name: "coreos-1.6.4",
							},
						},
					},
				}
			})

			Context("dns provider constraints", func() {
				It("should enforce that at least one provider has been defined", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.DNSProviders = []garden.DNSProviderConstraint{}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.dnsProviders", fldPath)),
					}))
				})

				It("should forbid unsupported providers", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.DNSProviders = invalidDNSProviders

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.dnsProviders[0]", fldPath)),
					}))
				})
			})

			Context("floating pools constraints", func() {
				It("should enforce that at least one pool has been defined", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.FloatingPools = []garden.OpenStackFloatingPool{}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(len(errorList)).To(Equal(1))
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

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.floatingPools[0].name", fldPath)),
					}))
				})
			})

			Context("kubernetes version constraints", func() {
				It("should enforce that at least one version has been defined", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.Kubernetes.Versions = []string{}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.versions", fldPath)),
					}))
				})

				It("should forbid versions of a not allowed pattern", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.Kubernetes.Versions = invalidKubernetes

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.kubernetes.versions[0]", fldPath)),
					}))
				})
			})

			Context("load balancer provider constraints", func() {
				It("should enforce that at least one provider has been defined", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.LoadBalancerProviders = []garden.OpenStackLoadBalancerProvider{}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(len(errorList)).To(Equal(1))
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

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.loadBalancerProviders[0].name", fldPath)),
					}))
				})
			})

			Context("machine types validation", func() {
				It("should enforce that at least one machine type has been defined", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.MachineTypes = []garden.MachineType{}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes", fldPath)),
					}))
				})

				It("should forbid machine types with unsupported property values", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.MachineTypes = invalidMachineTypes

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(len(errorList)).To(Equal(3))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].name", fldPath)),
					}))
					Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].cpus", fldPath)),
					}))
					Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.machineTypes[0].gpus", fldPath)),
					}))
				})
			})

			Context("zones validation", func() {
				It("should enforce that at least one zone has been defined", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.Zones = []garden.Zone{}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.constraints.zones", fldPath)),
					}))
				})

				It("should forbid zones with unsupported name values", func() {
					openStackCloudProfile.Spec.OpenStack.Constraints.Zones = invalidZones

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(len(errorList)).To(Equal(2))
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

			Context("machine image validation", func() {
				It("should forbid machine images with unsupported format", func() {
					openStackCloudProfile.Spec.OpenStack.MachineImage = garden.OpenStackMachineImage{
						Name: "",
					}

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.machineImage.name", fldPath)),
					}))
				})
			})

			Context("keystone url validation", func() {
				It("should forbid keystone urls with unsupported format", func() {
					openStackCloudProfile.Spec.OpenStack.KeyStoneURL = ""

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.keyStoneURL", fldPath)),
					}))
				})
			})

			Context("ca bundle validation", func() {
				It("should forbid ca bundles with unsupported format", func() {
					openStackCloudProfile.Spec.OpenStack.CABundle = "----"

					errorList := ValidateCloudProfile(openStackCloudProfile)

					Expect(len(errorList)).To(Equal(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal(fmt.Sprintf("spec.%s.caBundle", fldPath)),
					}))
				})
			})
		})
	})

	Describe("#ValidateSeed", func() {
		var seed *garden.Seed

		BeforeEach(func() {
			seed = &garden.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "seed-1",
				},
				Spec: garden.SeedSpec{
					Cloud: garden.SeedCloud{
						Profile: "aws",
						Region:  "eu-west-1",
					},
					Domain: "my-seed-1.example.com",
					SecretRef: garden.CrossReference{
						Name:      "seed-aws",
						Namespace: "garden",
					},
					Networks: garden.K8SNetworks{
						Nodes:    garden.CIDR("10.250.0.0/16"),
						Pods:     garden.CIDR("100.96.0.0/11"),
						Services: garden.CIDR("100.64.0.0/13"),
					},
				},
			}
		})

		It("should not return any errors", func() {
			errorList := ValidateSeed(seed)

			Expect(len(errorList)).To(Equal(0))
		})

		It("should forbid Seed resources with empty metadata", func() {
			seed.ObjectMeta = metav1.ObjectMeta{}

			errorList := ValidateSeed(seed)

			Expect(len(errorList)).To(Equal(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			}))
		})

		It("should forbid cloud specification with empty or invalid keys", func() {
			seed.Spec.Cloud = garden.SeedCloud{}
			seed.Spec.Domain = "invalid-domain-name"
			seed.Spec.SecretRef = garden.CrossReference{}
			seed.Spec.Networks = garden.K8SNetworks{
				Nodes:    garden.CIDR("invalid-cidr"),
				Pods:     garden.CIDR("300.300.300.300/300"),
				Services: garden.CIDR("invalid-cidr"),
			}

			errorList := ValidateSeed(seed)

			Expect(len(errorList)).To(Equal(8))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.cloud.profile"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.cloud.region"),
			}))
			Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.domain"),
			}))
			Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.secretRef.name"),
			}))
			Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.secretRef.namespace"),
			}))
			Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.networks.nodes"),
			}))
			Expect(*errorList[6]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.networks.pods"),
			}))
			Expect(*errorList[7]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.networks.services"),
			}))
		})

		It("should forbid too long seed domain names", func() {
			seed.Spec.Domain = "some.domain.name.which.is.too.long.to.be.valid.com"

			errorList := ValidateSeed(seed)

			Expect(len(errorList)).To(Equal(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeTooLong),
				"Field": Equal("spec.domain"),
			}))
		})
	})

	Describe("#ValidatePrivateSecretBinding", func() {
		var secretBinding *garden.PrivateSecretBinding

		BeforeEach(func() {
			secretBinding = &garden.PrivateSecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "profile",
					Namespace: "garden",
				},
				SecretRef: garden.LocalReference{
					Name: "my-secret",
				},
			}
		})

		It("should not return any errors", func() {
			errorList := ValidatePrivateSecretBinding(secretBinding)

			Expect(len(errorList)).To(Equal(0))
		})

		It("should forbid empty PrivateSecretBinding resources", func() {
			secretBinding.ObjectMeta = metav1.ObjectMeta{}
			secretBinding.SecretRef = garden.LocalReference{}

			errorList := ValidatePrivateSecretBinding(secretBinding)

			Expect(len(errorList)).To(Equal(3))
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
			secretBinding.Quotas = []garden.CrossReference{
				{},
			}

			errorList := ValidatePrivateSecretBinding(secretBinding)

			Expect(len(errorList)).To(Equal(2))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("quotas[0].name"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("quotas[0].namespace"),
			}))
		})
	})

	Describe("#ValidateCrossSecretBinding", func() {
		var secretBinding *garden.CrossSecretBinding

		BeforeEach(func() {
			secretBinding = &garden.CrossSecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "profile",
					Namespace: "garden",
				},
				SecretRef: garden.CrossReference{
					Name:      "my-secret",
					Namespace: "my-namespace",
				},
			}
		})

		It("should not return any errors", func() {
			errorList := ValidateCrossSecretBinding(secretBinding)

			Expect(len(errorList)).To(Equal(0))
		})

		It("should forbid empty CrossSecretBinding resources", func() {
			secretBinding.ObjectMeta = metav1.ObjectMeta{}
			secretBinding.SecretRef = garden.CrossReference{}

			errorList := ValidateCrossSecretBinding(secretBinding)

			Expect(len(errorList)).To(Equal(4))
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
			Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("secretRef.namespace"),
			}))
		})

		It("should forbid empty stated Quota names", func() {
			secretBinding.Quotas = []garden.CrossReference{
				{},
			}

			errorList := ValidateCrossSecretBinding(secretBinding)

			Expect(len(errorList)).To(Equal(2))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("quotas[0].name"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("quotas[0].namespace"),
			}))
		})
	})
})
