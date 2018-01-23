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
	corev1 "k8s.io/api/core/v1"
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
						"Type":  Equal(field.ErrorTypeInvalid),
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

		It("should forbid Seed specification with empty or invalid keys", func() {
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

	Describe("#ValidateQuota", func() {
		var quota *garden.Quota

		BeforeEach(func() {
			quota = &garden.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "quota-1",
					Namespace: "my-namespace",
				},
				Spec: garden.QuotaSpec{
					Scope: garden.QuotaScopeProject,
					Metrics: corev1.ResourceList{
						"cpus":   resource.MustParse("200"),
						"memory": resource.MustParse("4000Gi"),
					},
				},
			}
		})

		It("should not return any errors", func() {
			errorList := ValidateQuota(quota)

			Expect(len(errorList)).To(Equal(0))
		})

		It("should forbid Quota specification with empty or invalid keys", func() {
			quota.ObjectMeta = metav1.ObjectMeta{}
			quota.Spec.Scope = garden.QuotaScope("does-not-exist")
			quota.Spec.Metrics["key"] = resource.MustParse("-100")

			errorList := ValidateQuota(quota)

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
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.scope"),
			}))
			Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.metrics[key]"),
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

	Describe("#ValidateShoot, #ValidateShootUpdate", func() {
		var (
			shoot *garden.Shoot

			hostedZoneID = "ABCDEF1234"
			domain       = "my-cluster.example.com"

			invalidBackup = &garden.Backup{
				IntervalInSecond: 0,
				Maximum:          0,
			}
			addon = garden.Addon{
				Enabled: true,
			}
			k8sNetworks = garden.K8SNetworks{
				Nodes:    garden.CIDR("10.250.0.0/16"),
				Pods:     garden.CIDR("100.96.0.0/11"),
				Services: garden.CIDR("100.64.0.0/13"),
			}
			invalidK8sNetworks = garden.K8SNetworks{
				Nodes:    garden.CIDR("invalid-cidr"),
				Pods:     garden.CIDR("invalid-cidr"),
				Services: garden.CIDR("invalid-cidr"),
			}
			worker = garden.Worker{
				Name:          "worker-name",
				MachineType:   "large",
				AutoScalerMin: 1,
				AutoScalerMax: 1,
			}
			invalidWorker = garden.Worker{
				Name:          "",
				MachineType:   "",
				AutoScalerMin: -1,
				AutoScalerMax: -2,
			}
			invalidWorkerTooLongName = garden.Worker{
				Name:          "worker-name-is-too-long",
				MachineType:   "large",
				AutoScalerMin: 1,
				AutoScalerMax: 1,
			}

			makeStringPointer = func(s string) *string {
				ptr := s
				return &ptr
			}
		)

		BeforeEach(func() {
			shoot = &garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "my-namespace",
				},
				Spec: garden.ShootSpec{
					Addons: garden.Addons{
						Kube2IAM: garden.Kube2IAM{
							Addon: addon,
							Roles: []garden.Kube2IAMRole{
								{
									Name:        "iam-role",
									Description: "some-text",
									Policy:      `{"some-valid": "json-document"}`,
								},
							},
						},
						KubernetesDashboard: garden.KubernetesDashboard{
							Addon: addon,
						},
						ClusterAutoscaler: garden.ClusterAutoscaler{
							Addon: addon,
						},
						NginxIngress: garden.NginxIngress{
							Addon: addon,
						},
						Monocular: garden.Monocular{
							Addon: addon,
						},
						KubeLego: garden.KubeLego{
							Addon: addon,
							Mail:  "info@example.com",
						},
					},
					Backup: &garden.Backup{
						IntervalInSecond: 1,
						Maximum:          2,
					},
					Cloud: garden.Cloud{
						Profile: "aws-profile",
						Region:  "eu-west-1",
						SecretBindingRef: corev1.ObjectReference{
							Kind: "PrivateSecretBinding",
							Name: "my-secret",
						},
						AWS: &garden.AWSCloud{
							Networks: garden.AWSNetworks{
								K8SNetworks: k8sNetworks,
								Internal:    []garden.CIDR{"10.250.0.0/16"},
								Public:      []garden.CIDR{"10.250.0.0/16"},
								Workers:     []garden.CIDR{"10.250.0.0/16"},
								VPC: garden.AWSVPC{
									CIDR: garden.CIDR("10.250.0.0/16"),
								},
							},
							Workers: []garden.AWSWorker{
								{
									Worker:     worker,
									VolumeSize: "10Gi",
									VolumeType: "default",
								},
							},
							Zones: []string{"eu-west-1a"},
						},
					},
					DNS: garden.DNS{
						Provider:     garden.DNSAWSRoute53,
						HostedZoneID: &hostedZoneID,
						Domain:       &domain,
					},
					Kubernetes: garden.Kubernetes{
						KubeAPIServer: garden.KubeAPIServerConfig{
							OIDCConfig: &garden.OIDCConfig{
								CABundle:       makeStringPointer("-----BEGIN CERTIFICATE-----\nMIICRzCCAfGgAwIBAgIJALMb7ecMIk3MMA0GCSqGSIb3DQEBCwUAMH4xCzAJBgNV\nBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNVBAcMBkxvbmRvbjEYMBYGA1UE\nCgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1JVCBEZXBhcnRtZW50MRswGQYD\nVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTAwIBcNMTcwNDI2MjMyNjUyWhgPMjExNzA0\nMDIyMzI2NTJaMH4xCzAJBgNVBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNV\nBAcMBkxvbmRvbjEYMBYGA1UECgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1J\nVCBEZXBhcnRtZW50MRswGQYDVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTAwXDANBgkq\nhkiG9w0BAQEFAANLADBIAkEAtBMa7NWpv3BVlKTCPGO/LEsguKqWHBtKzweMY2CV\ntAL1rQm913huhxF9w+ai76KQ3MHK5IVnLJjYYA5MzP2H5QIDAQABo1AwTjAdBgNV\nHQ4EFgQU22iy8aWkNSxv0nBxFxerfsvnZVMwHwYDVR0jBBgwFoAU22iy8aWkNSxv\n0nBxFxerfsvnZVMwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAANBAEOefGbV\nNcHxklaW06w6OBYJPwpIhCVozC1qdxGX1dg8VkEKzjOzjgqVD30m59OFmSlBmHsl\nnkVA6wyOSDYBf3o=\n-----END CERTIFICATE-----"),
								ClientID:       makeStringPointer("client-id"),
								GroupsClaim:    makeStringPointer("groups-claim"),
								GroupsPrefix:   makeStringPointer("groups-prefix"),
								IssuerURL:      makeStringPointer("https://some-endpoint.com"),
								UsernameClaim:  makeStringPointer("user-claim"),
								UsernamePrefix: makeStringPointer("user-prefix"),
							},
						},
					},
				},
			}
		})

		It("should forbid empty Shoot resources", func() {
			shoot := &garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       garden.ShootSpec{},
			}

			errorList := ValidateShoot(shoot)

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
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.cloud.aws/azure/gcp/openstack"),
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

			errorList := ValidateShoot(shoot)

			Expect(len(errorList)).To(Equal(4))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.addons.kube2iam.roles[0].name"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.addons.kube2iam.roles[0].description"),
			}))
			Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.addons.kube2iam.roles[0].policy"),
			}))
			Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.addons.kube-lego.mail"),
			}))
		})

		It("should forbid unsupported cloud specification (provider independent)", func() {
			shoot.Spec.Cloud.Profile = ""
			shoot.Spec.Cloud.Region = ""
			shoot.Spec.Cloud.SecretBindingRef = corev1.ObjectReference{
				Kind: "unsupported",
				Name: "",
			}
			shoot.Spec.Cloud.Seed = makeStringPointer("")

			errorList := ValidateShoot(shoot)

			Expect(len(errorList)).To(Equal(5))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.cloud.profile"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.cloud.region"),
			}))
			Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.cloud.secretBindingRef.kind"),
			}))
			Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.cloud.secretBindingRef.name"),
			}))
			Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.cloud.seed"),
			}))
		})

		It("should forbid updating some cloud keys", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.Cloud.Profile = "another-profile"
			newShoot.Spec.Cloud.Region = "another-region"
			newShoot.Spec.Cloud.SecretBindingRef = corev1.ObjectReference{
				Kind: "PrivateSecretBinding",
				Name: "another-reference",
			}
			newShoot.Spec.Cloud.Seed = makeStringPointer("another-seed")

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(len(errorList)).To(Equal(4))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.cloud.profile"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.cloud.region"),
			}))
			Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.cloud.secretBindingRef"),
			}))
			Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.cloud.seed"),
			}))
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
						Internal:    []garden.CIDR{"10.250.0.0/16"},
						Public:      []garden.CIDR{"10.250.0.0/16"},
						Workers:     []garden.CIDR{"10.250.0.0/16"},
						VPC: garden.AWSVPC{
							CIDR: garden.CIDR("10.250.0.0/16"),
						},
					},
					Workers: []garden.AWSWorker{
						{
							Worker:     worker,
							VolumeSize: "10Gi",
							VolumeType: "default",
						},
					},
					Zones: []string{"eu-west-1a"},
				}
				shoot.Spec.Cloud.AWS = awsCloud
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(0))
			})

			It("should forbid invalid backup configuration", func() {
				shoot.Spec.Backup = invalidBackup

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.backup.intervalInSecond"),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.backup.maximum"),
				}))
			})

			It("should forbid invalid network configuration", func() {
				shoot.Spec.Cloud.AWS.Networks.Internal = []garden.CIDR{"one cidr", "another cidr"}
				shoot.Spec.Cloud.AWS.Networks.Public = []garden.CIDR{"invalid-cidr", "another cidr"}
				shoot.Spec.Cloud.AWS.Networks.Workers = []garden.CIDR{"invalid-cidr", "another cidr"}
				shoot.Spec.Cloud.AWS.Networks.K8SNetworks = invalidK8sNetworks
				shoot.Spec.Cloud.AWS.Networks.VPC = garden.AWSVPC{}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(13))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.nodes", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.pods", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.services", fldPath)),
				}))
				Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.internal", fldPath)),
				}))
				Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.internal[0]", fldPath)),
				}))
				Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.internal[1]", fldPath)),
				}))
				Expect(*errorList[6]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.public", fldPath)),
				}))
				Expect(*errorList[7]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.public[0]", fldPath)),
				}))
				Expect(*errorList[8]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.public[1]", fldPath)),
				}))
				Expect(*errorList[9]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
				}))
				Expect(*errorList[10]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers[0]", fldPath)),
				}))
				Expect(*errorList[11]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers[1]", fldPath)),
				}))
				Expect(*errorList[12]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.vpc", fldPath)),
				}))
			})

			It("should forbid invalid VPC CIDRs", func() {
				shoot.Spec.Cloud.AWS.Networks.VPC.CIDR = garden.CIDR("invalid")

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.vpc.cidr", fldPath)),
				}))
			})

			It("should require a node network when specifying an existing VPC", func() {
				shoot.Spec.Cloud.AWS.Networks.VPC = garden.AWSVPC{
					ID: "aws-vpc-id",
				}
				shoot.Spec.Cloud.AWS.Networks.Nodes = ""

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.nodes", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.nodes", fldPath)),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.AWSWorker{}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
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

				Expect(len(errorList)).To(Equal(7))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
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

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.AWS.Workers[0].Worker = invalidWorkerTooLongName

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid an empty zones list", func() {
				shoot.Spec.Cloud.AWS.Zones = []string{}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid updating networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.AWS.Networks.Pods = garden.CIDR("255.255.255.255/32")
				newShoot.Spec.Cloud.AWS.Zones = []string{"another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(len(errorList)).To(Equal(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid removing the AWS section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.AWS = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(len(errorList)).To(Equal(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.cloud.aws/azure/gcp/openstack"),
				}))
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
						Workers:     garden.CIDR("10.250.0.0/16"),
						VNet: garden.AzureVNet{
							CIDR: garden.CIDR("10.250.0.0/16"),
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

				Expect(len(errorList)).To(Equal(0))
			})

			It("should forbid invalid backup configuration", func() {
				shoot.Spec.Backup = invalidBackup

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.backup.intervalInSecond"),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.backup.maximum"),
				}))
			})

			It("should forbid specifying a resource group configuration", func() {
				shoot.Spec.Cloud.Azure.ResourceGroup = &garden.AzureResourceGroup{}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.resourceGroup.name", fldPath)),
				}))
			})

			It("should forbid specifying a vnet name", func() {
				shoot.Spec.Cloud.Azure.Networks.VNet = garden.AzureVNet{
					Name: "existing-vnet",
				}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.vnet.name", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.vnet.cidr", fldPath)),
				}))
			})

			It("should forbid invalid network configuration", func() {
				invalidCIDR := garden.CIDR("invalid-cidr")
				shoot.Spec.Cloud.Azure.Networks.Public = &invalidCIDR
				shoot.Spec.Cloud.Azure.Networks.Workers = invalidCIDR
				shoot.Spec.Cloud.Azure.Networks.K8SNetworks = invalidK8sNetworks
				shoot.Spec.Cloud.Azure.Networks.VNet = garden.AzureVNet{}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(6))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.vnet.cidr", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.nodes", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.pods", fldPath)),
				}))
				Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.services", fldPath)),
				}))
				Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.public", fldPath)),
				}))
				Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
				}))
			})

			It("should forbid invalid VNet CIDRs", func() {
				shoot.Spec.Cloud.Azure.Networks.VNet.CIDR = garden.CIDR("invalid")

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.vnet.cidr", fldPath)),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.AzureWorker{}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
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

				Expect(len(errorList)).To(Equal(8))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
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
				Expect(*errorList[7]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMax", fldPath)),
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

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volumeSize", fldPath)),
				}))
			})

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.Azure.Workers[0].Worker = invalidWorkerTooLongName

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid workers with auto scaling configured", func() {
				shoot.Spec.Cloud.Azure.Workers[0].AutoScalerMax = shoot.Spec.Cloud.Azure.Workers[0].AutoScalerMin + 1

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMax", fldPath)),
				}))
			})

			It("should forbid updating resource group and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.Azure.Networks.Pods = garden.CIDR("255.255.255.255/32")
				newShoot.Spec.Cloud.Azure.ResourceGroup = &garden.AzureResourceGroup{
					Name: "another-group",
				}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(len(errorList)).To(Equal(3))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.resourceGroup", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.resourceGroup.name", fldPath)),
				}))
			})

			It("should forbid removing the Azure section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.Azure = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(len(errorList)).To(Equal(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.cloud.aws/azure/gcp/openstack"),
				}))
			})
		})

		Context("GCP specific validation", func() {
			var (
				fldPath  = "gcp"
				gcpCloud *garden.GCPCloud
			)

			BeforeEach(func() {
				gcpCloud = &garden.GCPCloud{
					Networks: garden.GCPNetworks{
						K8SNetworks: k8sNetworks,
						Workers:     []garden.CIDR{"10.250.0.0/16"},
						VPC: &garden.GCPVPC{
							Name: "hugo",
						},
					},
					Workers: []garden.GCPWorker{
						{
							Worker:     worker,
							VolumeSize: "10Gi",
							VolumeType: "default",
						},
					},
					Zones: []string{"europe-west1-b"},
				}
				shoot.Spec.Cloud.AWS = nil
				shoot.Spec.Cloud.GCP = gcpCloud
				shoot.Spec.Backup = nil
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(0))
			})

			It("should forbid providing backup configuration", func() {
				shoot.Spec.Backup = &garden.Backup{}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.backup"),
				}))
			})

			It("should forbid invalid network configuration", func() {
				shoot.Spec.Cloud.GCP.Networks.Workers = []garden.CIDR{"invalid-cidr", "another cidr"}
				shoot.Spec.Cloud.GCP.Networks.K8SNetworks = invalidK8sNetworks
				shoot.Spec.Cloud.GCP.Networks.VPC = &garden.GCPVPC{}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(7))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.nodes", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.pods", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.services", fldPath)),
				}))
				Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
				}))
				Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers[0]", fldPath)),
				}))
				Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers[1]", fldPath)),
				}))
				Expect(*errorList[6]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.vpc.name", fldPath)),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.GCPWorker{}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
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

				Expect(len(errorList)).To(Equal(7))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
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

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.GCP.Workers[0].Worker = invalidWorkerTooLongName

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid an empty zones list", func() {
				shoot.Spec.Cloud.GCP.Zones = []string{}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid specifying more than one zone", func() {
				shoot.Spec.Cloud.GCP.Zones = []string{"zone1", "zone2"}
				shoot.Spec.Cloud.GCP.Networks.Workers = []garden.CIDR{"10.250.0.0/16", "10.250.0.0/16"}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid updating networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.GCP.Networks.Pods = garden.CIDR("255.255.255.255/32")
				newShoot.Spec.Cloud.GCP.Zones = []string{"another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(len(errorList)).To(Equal(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid removing the GCP section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.GCP = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(len(errorList)).To(Equal(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.cloud.aws/azure/gcp/openstack"),
				}))
			})
		})

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
				shoot.Spec.Backup = nil
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(0))
			})

			It("should forbid providing backup configuration", func() {
				shoot.Spec.Backup = &garden.Backup{}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.backup"),
				}))
			})

			It("should forbid invalid floating pool name configuration", func() {
				shoot.Spec.Cloud.OpenStack.FloatingPoolName = ""

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.floatingPoolName", fldPath)),
				}))
			})

			It("should forbid invalid load balancer provider configuration", func() {
				shoot.Spec.Cloud.OpenStack.LoadBalancerProvider = ""

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.loadBalancerProvider", fldPath)),
				}))
			})

			It("should forbid invalid network configuration", func() {
				shoot.Spec.Cloud.OpenStack.Networks.Workers = []garden.CIDR{"invalid-cidr", "another cidr"}
				shoot.Spec.Cloud.OpenStack.Networks.K8SNetworks = invalidK8sNetworks
				shoot.Spec.Cloud.OpenStack.Networks.Router = &garden.OpenStackRouter{}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(7))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.nodes", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.pods", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.services", fldPath)),
				}))
				Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
				}))
				Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers[0]", fldPath)),
				}))
				Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers[1]", fldPath)),
				}))
				Expect(*errorList[6]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.router.id", fldPath)),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.OpenStack.Workers = []garden.OpenStackWorker{}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
				}))
			})

			It("should forbid invalid worker configuration", func() {
				shoot.Spec.Cloud.OpenStack.Workers = []garden.OpenStackWorker{
					{
						Worker: invalidWorker,
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(6))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
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
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].autoScalerMax", fldPath)),
				}))
			})

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.OpenStack.Workers = []garden.OpenStackWorker{
					{
						Worker: invalidWorkerTooLongName,
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid an empty zones list", func() {
				shoot.Spec.Cloud.OpenStack.Zones = []string{}

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid updating networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.OpenStack.Networks.Pods = garden.CIDR("255.255.255.255/32")
				newShoot.Spec.Cloud.OpenStack.Zones = []string{"another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(len(errorList)).To(Equal(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid removing the OpenStack section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.OpenStack = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(len(errorList)).To(Equal(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.cloud.aws/azure/gcp/openstack"),
				}))
			})
		})

		Context("dns section", func() {
			It("should forbid unsupported dns providers", func() {
				shoot.Spec.DNS.Provider = garden.DNSProvider("does-not-exist")

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.dns.provider"),
				}))
			})

			It("should forbid empty hosted zone ids or domains", func() {
				shoot.Spec.DNS.HostedZoneID = makeStringPointer("")
				shoot.Spec.DNS.Domain = makeStringPointer("")

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(2))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.hostedZoneID"),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.domain"),
				}))
			})

			It("should forbid not specifying a domain when provider not equals 'unmanaged'", func() {
				shoot.Spec.DNS.Provider = garden.DNSAWSRoute53
				shoot.Spec.DNS.Domain = nil

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.dns.domain"),
				}))
			})

			It("should forbid monocular when provider equals 'unmanaged'", func() {
				shoot.Spec.DNS.Provider = garden.DNSUnmanaged
				shoot.Spec.DNS.HostedZoneID = nil

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.addons.monocular.enabled"),
				}))
			})

			It("should forbid specifying a hosted zone id when provider equals 'unmanaged'", func() {
				shoot.Spec.DNS.Provider = garden.DNSUnmanaged
				shoot.Spec.Addons.Monocular.Enabled = false

				errorList := ValidateShoot(shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.hostedZoneID"),
				}))
			})

			It("should forbid updating the dns section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.DNS.Domain = makeStringPointer("another-domain.com")
				newShoot.Spec.DNS.HostedZoneID = makeStringPointer("another-hosted-zone")
				newShoot.Spec.DNS.Provider = garden.DNSAWSRoute53

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(len(errorList)).To(Equal(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns"),
				}))
			})
		})

		It("should forbid unsupported kubernetes configuration", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.CABundle = makeStringPointer("")
			shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = makeStringPointer("")
			shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.GroupsClaim = makeStringPointer("")
			shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.GroupsPrefix = makeStringPointer("")
			shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.IssuerURL = makeStringPointer("")
			shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.UsernameClaim = makeStringPointer("")
			shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.UsernamePrefix = makeStringPointer("")

			errorList := ValidateShoot(shoot)

			Expect(len(errorList)).To(Equal(7))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.caBundle"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.clientID"),
			}))
			Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.groupsClaim"),
			}))
			Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.groupsPrefix"),
			}))
			Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.issuerURL"),
			}))
			Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.usernameClaim"),
			}))
			Expect(*errorList[6]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.usernamePrefix"),
			}))
		})

		It("should forbid updating the kubernetes version", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.Kubernetes.Version = "2.0.0"

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(len(errorList)).To(Equal(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.kubernetes.version"),
			}))
		})
	})
})

// Helper functions

func prepareShootForUpdate(shoot *garden.Shoot) *garden.Shoot {
	s := shoot.DeepCopy()
	s.ResourceVersion = "1"
	return s
}
