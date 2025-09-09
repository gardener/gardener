// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	. "github.com/gardener/gardener/pkg/provider-local/apis/local/validation"
)

var _ = Describe("CloudProfileConfig validation", func() {
	var (
		cloudProfileConfig      *api.CloudProfileConfig
		machineImages           []core.MachineImage
		capabilitiesDefinitions []v1beta1.CapabilityDefinition
		imageString             string
		fldPath                 = field.NewPath("spec")
	)

	BeforeEach(func() {
		imageString = "some-image-reference"
		cloudProfileConfig = &api.CloudProfileConfig{
			MachineImages: []api.MachineImages{
				{
					Name: "ubuntu",
					Versions: []api.MachineImageVersion{
						{
							Version: "18.04",
							Flavors: []api.MachineImageFlavor{
								{
									Image: imageString,
									Capabilities: v1beta1.Capabilities{
										v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
									},
								},
								{
									Image: imageString,
									Capabilities: v1beta1.Capabilities{
										v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureARM64},
									},
								},
							},
						},
					},
				},
			},
		}

		machineImages = []core.MachineImage{
			{
				Name: "ubuntu",
				Versions: []core.MachineImageVersion{
					{
						ExpirableVersion: core.ExpirableVersion{
							Version: "18.04",
						},
						Flavors: []core.MachineImageFlavor{{
							Capabilities: core.Capabilities{
								v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
							},
						}, {
							Capabilities: core.Capabilities{
								v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureARM64},
							},
						}},
					},
				},
			},
		}

		capabilitiesDefinitions = []v1beta1.CapabilityDefinition{
			{
				Name:   v1beta1constants.ArchitectureName,
				Values: []string{v1beta1constants.ArchitectureAMD64, v1beta1constants.ArchitectureARM64},
			},
			{
				Name:   "cap1",
				Values: []string{"value1", "value2"},
			},
		}
	})

	Describe("#ValidateCloudProfileConfig", func() {
		Context("basic validation", func() {
			It("should succeed with valid configuration", func() {
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(BeEmpty())
			})

			It("should fail with empty machine images", func() {
				cloudProfileConfig.MachineImages = []api.MachineImages{}
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.machineImages"),
					"Detail": ContainSubstring("must provide at least one machine image"),
				}))))
			})

			It("should fail with empty machine image name", func() {
				cloudProfileConfig.MachineImages[0].Name = ""
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.machineImages[0].name"),
				}))))
			})

			It("should fail with empty versions", func() {
				cloudProfileConfig.MachineImages[0].Versions = []api.MachineImageVersion{}
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.machineImages[0].versions"),
				}))))
			})

			It("should fail with empty version string", func() {
				cloudProfileConfig.MachineImages[0].Versions[0].Version = ""
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.machineImages[0].versions[0].version"),
				}))))
			})
		})

		Context("with no capabilities defined", func() {
			BeforeEach(func() {
				// set capability sets to empty to simulate no capabilities defined
				cloudProfileConfig.MachineImages[0].Versions[0].Flavors = []api.MachineImageFlavor{}
				cloudProfileConfig.MachineImages[0].Versions[0].Image = "ubuntu-18.04-amd64"
				machineImages[0].Versions[0].Flavors = []core.MachineImageFlavor{}
				capabilitiesDefinitions = []v1beta1.CapabilityDefinition{}
			})

			It("should succeed with valid configuration", func() {
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(BeEmpty())
			})

			It("should fail with empty image string", func() {
				cloudProfileConfig.MachineImages[0].Versions[0].Image = ""
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.machineImages[0].versions[0].image"),
				}))))
			})

			It("should fail when machine image doesn't exist in provider config", func() {
				machineImages[0].Name = "debian"
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.machineImages[0]"),
					"Detail": ContainSubstring("debian"),
				}))))
			})
		})

		Context("with capabilities", func() {
			It("should succeed with valid capability sets", func() {
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(BeEmpty())
			})

			It("should fail if image string is set when using capabilities", func() {
				cloudProfileConfig.MachineImages[0].Versions[0].Image = "ubuntu-18.04"
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.machineImages[0].versions[0].image"),
				}))))
			})

			It("should fail if capability sets contain invalid capability", func() {
				cloudProfileConfig.MachineImages[0].Versions[0].Flavors[0].Capabilities["invalid"] = []string{"value"}
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.machineImages[0].versions[0].capabilitySets[0].capabilities"),
				}))))
			})

			It("should fail when machine image version doesn't exist in provider config with capabilities", func() {
				machineImages[0].Versions[0].Version = "20.04"
				machineImages[0].Versions[0].Flavors = []core.MachineImageFlavor{
					{
						Capabilities: core.Capabilities{
							v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
						},
					},
				}

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.machineImages[0].versions[0]"),
					"Detail": ContainSubstring("ubuntu@20.04"),
				}))))
			})

			It("should fail when capability set has empty image", func() {
				cloudProfileConfig.MachineImages[0].Versions[0].Flavors[0].Image = ""
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.machineImages[0].versions[0].capabilitySets[0].image"),
				}))))
			})

			It("should fail when capability values are not from defined set", func() {
				cloudProfileConfig.MachineImages[0].Versions[0].Flavors[0].Capabilities["cap1"] = []string{"invalid-value"}
				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeNotSupported),
					"Field":  Equal("spec.machineImages[0].versions[0].capabilitySets[0].capabilities.cap1[0]"),
					"Detail": ContainSubstring("supported values: \"value1\", \"value2\""),
				}))))
			})

			It("should fail when machine image version exists in core but not in provider config", func() {
				machineImages[0].Versions = append(machineImages[0].Versions, core.MachineImageVersion{
					ExpirableVersion: core.ExpirableVersion{Version: "20.04"},
					Flavors: []core.MachineImageFlavor{{
						Capabilities: core.Capabilities{
							v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
						},
					}},
				})

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.machineImages[0].versions[1]"),
					"Detail": ContainSubstring("ubuntu@20.04"),
				}))))
			})

			It("should fail when core capability set has no matching provider capability set", func() {
				machineImages[0].Versions[0].Flavors = append(machineImages[0].Versions[0].Flavors,
					core.MachineImageFlavor{
						Capabilities: core.Capabilities{
							"cap1": []string{"value1"},
						},
					})

				errorList := ValidateCloudProfileConfig(cloudProfileConfig, machineImages, capabilitiesDefinitions, fldPath)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.machineImages[0].versions[0].capabilitySets[2]"),
					"Detail": ContainSubstring("missing providerConfig mapping"),
				}))))
			})
		})
	})
})
