// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/provider-local/apis/local"
)

var _ = Describe("Helper Functions", func() {

	Context("#FindImageFromCloudProfile", func() {
		var (
			cloudProfileConfig    *local.CloudProfileConfig
			imageName             string
			imageVersion          string
			latestImageVersion    string
			machineCapabilities   v1beta1.Capabilities
			capabilityDefinitions []v1beta1.CapabilityDefinition
			suffixOne             string
			suffixTwo             string
		)

		BeforeEach(func() {
			imageName = "test-image"
			imageVersion = "1.0.0"
			latestImageVersion = "1.0.1"
			suffixOne = "-capability-set-1"
			suffixTwo = "-capability-set-2"

			machineCapabilities = v1beta1.Capabilities{
				v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
			}

			capabilityDefinitions = []v1beta1.CapabilityDefinition{
				{
					Name:   v1beta1constants.ArchitectureName,
					Values: []string{v1beta1constants.ArchitectureAMD64, v1beta1constants.ArchitectureARM64},
				},
				{
					Name:   "cap1",
					Values: []string{"value1", "value2", "value3"},
				},
				{
					Name:   "cap2",
					Values: []string{"valueA", "valueB", "valueC"},
				},
			}

			cloudProfileConfig = &local.CloudProfileConfig{
				MachineImages: []local.MachineImages{
					{
						Name: imageName,
						Versions: []local.MachineImageVersion{
							{
								Version: imageVersion,
								CapabilitySets: []local.CapabilitySet{
									{
										Image: imageVersion + suffixOne,
										Capabilities: core.Capabilities{
											v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
											"cap1":                            []string{"value1"},
										},
									},
									{
										Image: imageVersion + suffixTwo,
										Capabilities: core.Capabilities{
											v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
											"cap1":                            []string{"value2"},
										},
									},
								},
							},
							{
								Version: latestImageVersion,
								CapabilitySets: []local.CapabilitySet{
									{
										Image: latestImageVersion + suffixOne,
										Capabilities: core.Capabilities{
											v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureARM64},
										},
									},
									{
										Image: latestImageVersion + suffixTwo,
										Capabilities: core.Capabilities{
											v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
										},
									},
								},
							},
						},
					},
				},
			}

		})

		It("should find image when capabilities are matching exactly one capabilitySet", func() {
			machineCapabilities["cap1"] = []string{"value2"}

			image, err := FindImageFromCloudProfile(cloudProfileConfig, imageName, imageVersion, machineCapabilities, capabilityDefinitions)

			Expect(err).NotTo(HaveOccurred())
			Expect(image.Image).To(Equal(imageVersion + suffixTwo))
		})

		It("should return error when no capability set matches; this indicates a bug in the cloudProfile validation of the provider extension", func() {
			// Add cap1 with value3 which doesn't exist in any capability set
			machineCapabilities["cap1"] = []string{"value3"}

			image, err := FindImageFromCloudProfile(cloudProfileConfig, imageName, imageVersion, machineCapabilities, capabilityDefinitions)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(fmt.Sprintf("could not find image %q, version %q that supports %v: could not determine best capabilitySet no compatible capability set found", imageName, imageVersion, machineCapabilities)))
			Expect(image).To(BeNil())
		})

		Context("Multiple capability sets are viable matches", func() {
			// When multiple capability sets match the requirements for the machineType, selection follows a priority-based approach:
			//
			// 1. Capability sets are evaluated based on their supported capabilities
			// 2. Capabilities are ordered by priority in the definitions list (highest priority first)
			// 3. Within each capability, values are ordered by preference (most preferred first)
			// 4. Selection is determined by the first capability value difference found
			// +------------+-----------+-----------+-----------+-----------+
			// | Name       | Value 1   | Value 2   | ...       | Value N   |
			// +------------+-----------+-----------+-----------+-----------+
			// | Cap-1     	| prio-1  	| prio-2    | ...       | pio-n     |
			// | Cap-2     	| prio-n+1 	| prio-n+2  | ...       | prio-2n   |
			// | ...       	| ...      	| ...       | ...       | ...       |
			// | Cap-X     	| prio-xn+1	| prio-xn+2 | ...       | prio-xn+n |
			// +------------+-----------+-----------+-----------+-----------+

			It("should find image based on capability order", func() {
				cloudProfileConfig.MachineImages[0].Versions[1].CapabilitySets = []local.CapabilitySet{
					{
						Image: latestImageVersion + suffixOne,
						Capabilities: core.Capabilities{
							v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
							"cap1":                            []string{"value2"}, // Lower priority
							"cap2":                            []string{"valueA"},
						},
					},
					{
						Image: latestImageVersion + suffixTwo,
						Capabilities: core.Capabilities{
							v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
							"cap1":                            []string{"value1"}, // Higher priority (should be selected)
							"cap2":                            []string{"valueB"}, // cap2 should not affect decision as cap1 already differentiates
						},
					},
				}

				image, err := FindImageFromCloudProfile(cloudProfileConfig, imageName, latestImageVersion, machineCapabilities, capabilityDefinitions)

				Expect(err).NotTo(HaveOccurred())
				Expect(image.Image).To(Equal(latestImageVersion + suffixTwo))
			})

			It("should select image based on capability value priority within one capability", func() {
				// Set up two capability sets with different value orders for cap2
				cloudProfileConfig.MachineImages[0].Versions[1].CapabilitySets = []local.CapabilitySet{
					{
						Image: latestImageVersion + suffixOne,
						Capabilities: core.Capabilities{
							v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
							"cap2":                            []string{"valueA", "valueB"}, // valueB is higher priority
						},
					},
					{
						Image: latestImageVersion + suffixTwo,
						Capabilities: core.Capabilities{
							v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
							"cap2":                            []string{"valueA", "valueC"}, // valueC is lower priority than valueB
						},
					},
				}

				image, err := FindImageFromCloudProfile(cloudProfileConfig, imageName, latestImageVersion, machineCapabilities, capabilityDefinitions)

				Expect(err).NotTo(HaveOccurred())
				Expect(image.Image).To(Equal(latestImageVersion + suffixOne))
			})
		})

		Context("when handling edge cases", func() {
			It("should handle multiple capability sets with identical capabilities", func() {
				// Both capability sets have identical capabilities - this should be considered an error
				cloudProfileConfig.MachineImages[0].Versions[1].CapabilitySets = []local.CapabilitySet{
					{
						Image: latestImageVersion + suffixOne,
						Capabilities: core.Capabilities{
							v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
						},
					},
					{
						Image: latestImageVersion + suffixTwo,
						Capabilities: core.Capabilities{
							v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64},
						},
					},
				}

				_, err := FindImageFromCloudProfile(cloudProfileConfig, imageName, latestImageVersion, machineCapabilities, capabilityDefinitions)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("found multiple capability sets with identical capabilities"))
			})

			It("should return error for non-existent image name", func() {
				image, err := FindImageFromCloudProfile(cloudProfileConfig, "nonexistent-image", imageVersion, machineCapabilities, capabilityDefinitions)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find image"))
				Expect(image).To(BeNil())
			})

			It("should return error for non-existent image version", func() {
				image, err := FindImageFromCloudProfile(cloudProfileConfig, imageName, "nonexistent-version", machineCapabilities, capabilityDefinitions)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not find image"))
				Expect(image).To(BeNil())
			})

			It("should handle nil cloud profile config", func() {
				image, err := FindImageFromCloudProfile(nil, imageName, imageVersion, machineCapabilities, capabilityDefinitions)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cloud profile config is nil"))
				Expect(image).To(BeNil())
			})
		})
	})
})
