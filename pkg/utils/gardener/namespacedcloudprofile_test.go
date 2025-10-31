// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("TransformSpecToParentFormat", func() {
	When("parent uses capabilities", func() {
		var capabilityDefinitions []gardencorev1beta1.CapabilityDefinition

		BeforeEach(func() {
			capabilityDefinitions = []gardencorev1beta1.CapabilityDefinition{
				{
					Name:   v1beta1constants.ArchitectureName,
					Values: []string{"amd64", "arm64"},
				},
			}
		})

		It("should transform legacy architectures to capability flavors", func() {
			spec := gardencorev1beta1.NamespacedCloudProfileSpec{
				MachineImages: []gardencorev1beta1.MachineImage{
					{
						Name: "ubuntu",
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: "20.04",
								},
								Architectures: []string{"amd64", "arm64"},
							},
						},
					},
				},
				MachineTypes: []gardencorev1beta1.MachineType{
					{
						Name:         "m5.large",
						CPU:          resource.MustParse("2"),
						Memory:       resource.MustParse("8Gi"),
						Architecture: ptr.To("amd64"),
					},
				},
			}

			result := TransformSpecToParentFormat(spec, capabilityDefinitions)

			Expect(result.MachineImages).To(HaveLen(1))
			Expect(result.MachineImages[0].Versions).To(HaveLen(1))
			version := result.MachineImages[0].Versions[0]
			Expect(version.Architectures).To(Equal([]string{"amd64", "arm64"}))
			Expect(version.CapabilityFlavors).To(HaveLen(2))
			Expect(version.CapabilityFlavors[0].Capabilities[v1beta1constants.ArchitectureName]).To(BeEquivalentTo([]string{"amd64"}))
			Expect(version.CapabilityFlavors[1].Capabilities[v1beta1constants.ArchitectureName]).To(BeEquivalentTo([]string{"arm64"}))

			Expect(result.MachineTypes).To(HaveLen(1))
			machineType := result.MachineTypes[0]
			Expect(machineType.Architecture).To(Equal(ptr.To("amd64")))
			Expect(machineType.Capabilities[v1beta1constants.ArchitectureName]).To(BeEquivalentTo([]string{"amd64"}))
		})

		It("should preserve existing capability flavors when already present", func() {
			spec := gardencorev1beta1.NamespacedCloudProfileSpec{
				MachineImages: []gardencorev1beta1.MachineImage{
					{
						Name: "ubuntu",
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: "20.04",
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{
										Capabilities: gardencorev1beta1.Capabilities{
											v1beta1constants.ArchitectureName: []string{"amd64"},
										},
									},
								},
							},
						},
					},
				},
				MachineTypes: []gardencorev1beta1.MachineType{
					{
						Name:   "m5.large",
						CPU:    resource.MustParse("2"),
						Memory: resource.MustParse("8Gi"),
						Capabilities: gardencorev1beta1.Capabilities{
							v1beta1constants.ArchitectureName: []string{"amd64"},
						},
					},
				},
			}

			result := TransformSpecToParentFormat(spec, capabilityDefinitions)

			Expect(result.MachineImages[0].Versions[0].CapabilityFlavors).To(HaveLen(1))
			Expect(result.MachineImages[0].Versions[0].CapabilityFlavors[0].Capabilities[v1beta1constants.ArchitectureName]).To(BeEquivalentTo([]string{"amd64"}))
			Expect(result.MachineTypes[0].Capabilities[v1beta1constants.ArchitectureName]).To(BeEquivalentTo([]string{"amd64"}))
		})

		It("should default to AMD64 for machine types without architecture", func() {
			spec := gardencorev1beta1.NamespacedCloudProfileSpec{
				MachineTypes: []gardencorev1beta1.MachineType{
					{
						Name:   "m5.large",
						CPU:    resource.MustParse("2"),
						Memory: resource.MustParse("8Gi"),
					},
				},
			}

			result := TransformSpecToParentFormat(spec, capabilityDefinitions)

			Expect(result.MachineTypes[0].Capabilities[v1beta1constants.ArchitectureName]).To(BeEquivalentTo([]string{"amd64"}))
		})
	})

	When("parent doesn't use capabilities", func() {
		It("should transform capability flavors to legacy architectures", func() {
			spec := gardencorev1beta1.NamespacedCloudProfileSpec{
				MachineImages: []gardencorev1beta1.MachineImage{
					{
						Name: "ubuntu",
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: "20.04",
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
									{
										Capabilities: gardencorev1beta1.Capabilities{
											v1beta1constants.ArchitectureName: []string{"amd64"},
										},
									},
									{
										Capabilities: gardencorev1beta1.Capabilities{
											v1beta1constants.ArchitectureName: []string{"arm64"},
										},
									},
								},
							},
						},
					},
				},
				MachineTypes: []gardencorev1beta1.MachineType{
					{
						Name:   "m5.large",
						CPU:    resource.MustParse("2"),
						Memory: resource.MustParse("8Gi"),
						Capabilities: gardencorev1beta1.Capabilities{
							v1beta1constants.ArchitectureName: []string{"amd64"},
						},
					},
				},
			}

			result := TransformSpecToParentFormat(spec, nil)

			Expect(result.MachineImages[0].Versions[0].Architectures).To(ConsistOf("amd64", "arm64"))
			Expect(result.MachineImages[0].Versions[0].CapabilityFlavors).To(BeNil())
			Expect(result.MachineTypes[0].Capabilities).To(BeNil())
			Expect(result.MachineTypes[0].Architecture).To(Equal(ptr.To("amd64")))
		})

		It("should leave architectures empty for machine image versions with empty capability flavors, to not overwrite images during deepMerge", func() {
			spec := gardencorev1beta1.NamespacedCloudProfileSpec{
				MachineImages: []gardencorev1beta1.MachineImage{
					{
						Name: "ubuntu",
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: "20.04",
								},
								CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{},
							},
						},
					},
				},
			}

			result := TransformSpecToParentFormat(spec, nil)

			Expect(result.MachineImages[0].Versions[0].Architectures).To(BeEmpty())
		})
	})

	It("should handle multiple machine images and types correctly", func() {
		spec := gardencorev1beta1.NamespacedCloudProfileSpec{
			MachineImages: []gardencorev1beta1.MachineImage{
				{
					Name: "ubuntu",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "20.04",
							},
							Architectures: []string{"amd64"},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "22.04",
							},
							Architectures: []string{"arm64"},
						},
					},
				},
				{
					Name: "debian",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "11",
							},
							Architectures: []string{"amd64", "arm64"},
						},
					},
				},
			},
			MachineTypes: []gardencorev1beta1.MachineType{
				{
					Name:         "m5.large",
					CPU:          resource.MustParse("2"),
					Memory:       resource.MustParse("8Gi"),
					Architecture: ptr.To("amd64"),
				},
				{
					Name:         "m6g.large",
					CPU:          resource.MustParse("2"),
					Memory:       resource.MustParse("8Gi"),
					Architecture: ptr.To("arm64"),
				},
			},
		}

		capabilityDefinitions := []gardencorev1beta1.CapabilityDefinition{
			{
				Name:   v1beta1constants.ArchitectureName,
				Values: []string{"amd64", "arm64"},
			},
		}

		result := TransformSpecToParentFormat(spec, capabilityDefinitions)

		// Verify all machine images have capability flavors
		Expect(result.MachineImages).To(ConsistOf(gardencorev1beta1.MachineImage{
			Name: "ubuntu",
			Versions: []gardencorev1beta1.MachineImageVersion{
				{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "20.04"},
					Architectures:    []string{"amd64"},
					CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
						{
							Capabilities: gardencorev1beta1.Capabilities{
								"architecture": {"amd64"},
							},
						},
					},
				},
				{
					ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "22.04"},
					Architectures:    []string{"arm64"},
					CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
						{
							Capabilities: gardencorev1beta1.Capabilities{
								"architecture": {"arm64"},
							},
						},
					},
				},
			},
		},
			gardencorev1beta1.MachineImage{
				Name: "debian",
				Versions: []gardencorev1beta1.MachineImageVersion{
					{
						ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "11"},
						Architectures:    []string{"amd64", "arm64"},
						CapabilityFlavors: []gardencorev1beta1.MachineImageFlavor{
							{
								Capabilities: gardencorev1beta1.Capabilities{
									"architecture": {"amd64"},
								},
							},
							{
								Capabilities: gardencorev1beta1.Capabilities{
									"architecture": {"arm64"},
								},
							},
						},
					},
				},
			}))
		// Verify all machine types have capabilities
		Expect(result.MachineTypes).To(Equal([]gardencorev1beta1.MachineType{
			{
				Name:         "m5.large",
				CPU:          resource.MustParse("2"),
				Memory:       resource.MustParse("8Gi"),
				Architecture: ptr.To("amd64"),
				Capabilities: gardencorev1beta1.Capabilities{
					"architecture": {"amd64"},
				},
			},
			{
				Name:         "m6g.large",
				CPU:          resource.MustParse("2"),
				Memory:       resource.MustParse("8Gi"),
				Architecture: ptr.To("arm64"),
				Capabilities: gardencorev1beta1.Capabilities{
					"architecture": {"arm64"},
				},
			},
		}))
	})

	It("should return empty spec for empty input", func() {
		spec := gardencorev1beta1.NamespacedCloudProfileSpec{}
		capabilityDefinitions := []gardencorev1beta1.CapabilityDefinition{
			{
				Name:   v1beta1constants.ArchitectureName,
				Values: []string{"amd64", "arm64"},
			},
		}

		result := TransformSpecToParentFormat(spec, capabilityDefinitions)

		Expect(result).To(Equal(gardencorev1beta1.NamespacedCloudProfileSpec{}))
	})

	It("should not modify the original spec (deep copy behavior)", func() {
		originalSpec := gardencorev1beta1.NamespacedCloudProfileSpec{
			MachineImages: []gardencorev1beta1.MachineImage{
				{
					Name: "ubuntu",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "20.04",
							},
							Architectures: []string{"amd64"},
						},
					},
				},
			},
		}

		capabilityDefinitions := []gardencorev1beta1.CapabilityDefinition{
			{
				Name:   v1beta1constants.ArchitectureName,
				Values: []string{"amd64", "arm64"},
			},
		}

		// Keep references to verify original is unchanged
		originalArchitectures := originalSpec.MachineImages[0].Versions[0].Architectures
		originalCapabilityFlavors := originalSpec.MachineImages[0].Versions[0].CapabilityFlavors

		result := TransformSpecToParentFormat(originalSpec, capabilityDefinitions)

		// Original should remain unchanged
		Expect(originalSpec.MachineImages[0].Versions[0].Architectures).To(Equal(originalArchitectures))
		Expect(originalSpec.MachineImages[0].Versions[0].CapabilityFlavors).To(Equal(originalCapabilityFlavors))

		// Result should have capability flavors added
		Expect(result.MachineImages[0].Versions[0].CapabilityFlavors).ToNot(BeNil())
		Expect(result.MachineImages[0].Versions[0].CapabilityFlavors).To(HaveLen(1))
	})
})
