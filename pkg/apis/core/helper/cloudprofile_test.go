// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/helper"
)

var _ = Describe("CloudProfile Helper", func() {
	Describe("#CurrentLifecycleClassification", func() {
		It("version is implicitly supported", func() {
			classification := CurrentLifecycleClassification(core.ExpirableVersion{
				Version: "1.33.0",
			})
			Expect(classification).To(Equal(core.ClassificationSupported))
		})

		It("version is explicitly supported", func() {
			classification := CurrentLifecycleClassification(core.ExpirableVersion{
				Version:        "1.33.0",
				Classification: ptr.To(core.ClassificationSupported),
			})
			Expect(classification).To(Equal(core.ClassificationSupported))
		})

		It("version is in preview stage", func() {
			classification := CurrentLifecycleClassification(core.ExpirableVersion{
				Version:        "1.33.0",
				Classification: ptr.To(core.ClassificationPreview),
			})
			Expect(classification).To(Equal(core.ClassificationPreview))
		})

		It("version is deprecated ", func() {
			classification := CurrentLifecycleClassification(core.ExpirableVersion{
				Version:        "1.33.0",
				Classification: ptr.To(core.ClassificationDeprecated),
			})
			Expect(classification).To(Equal(core.ClassificationDeprecated))
		})

		It("supported version will expire in the future", func() {
			classification := CurrentLifecycleClassification(core.ExpirableVersion{
				Version:        "1.33.0",
				Classification: ptr.To(core.ClassificationSupported),
				ExpirationDate: ptr.To(metav1.NewTime(time.Now().Add(2 * time.Hour))),
			})
			Expect(classification).To(Equal(core.ClassificationSupported))
		})

		It("supported version has already expired", func() {
			classification := CurrentLifecycleClassification(core.ExpirableVersion{
				Version:        "1.33.0",
				Classification: ptr.To(core.ClassificationSupported),
				ExpirationDate: ptr.To(metav1.NewTime(time.Now().Add(-2 * time.Hour))),
			})
			Expect(classification).To(Equal(core.ClassificationExpired))
		})
	})

	Describe("#FindMachineImageVersion", func() {
		var machineImages []core.MachineImage

		BeforeEach(func() {
			machineImages = []core.MachineImage{
				{
					Name: "coreos",
					Versions: []core.MachineImageVersion{
						{
							ExpirableVersion: core.ExpirableVersion{
								Version: "0.0.2",
							},
						},
						{
							ExpirableVersion: core.ExpirableVersion{
								Version: "0.0.3",
							},
						},
					},
				},
			}
		})

		It("should find the machine image version when it exists", func() {
			expected := core.MachineImageVersion{
				ExpirableVersion: core.ExpirableVersion{
					Version: "0.0.3",
				},
			}

			actual, ok := FindMachineImageVersion(machineImages, "coreos", "0.0.3")
			Expect(ok).To(BeTrue())
			Expect(actual).To(Equal(expected))
		})

		It("should return false when machine image with the given name does not exist", func() {
			actual, ok := FindMachineImageVersion(machineImages, "foo", "0.0.3")
			Expect(ok).To(BeFalse())
			Expect(actual).To(Equal(core.MachineImageVersion{}))
		})

		It("should return false when machine image version with the given version does not exist", func() {
			actual, ok := FindMachineImageVersion(machineImages, "coreos", "0.0.4")
			Expect(ok).To(BeFalse())
			Expect(actual).To(Equal(core.MachineImageVersion{}))
		})
	})

	classificationPreview := core.ClassificationPreview
	classificationDeprecated := core.ClassificationDeprecated
	classificationSupported := core.ClassificationSupported
	previewVersion := core.MachineImageVersion{
		ExpirableVersion: core.ExpirableVersion{
			Version:        "1.1.2",
			Classification: &classificationPreview,
		},
	}
	deprecatedVersion := core.MachineImageVersion{
		ExpirableVersion: core.ExpirableVersion{
			Version:        "1.1.1",
			Classification: &classificationDeprecated,
		},
	}
	supportedVersion := core.MachineImageVersion{
		ExpirableVersion: core.ExpirableVersion{
			Version:        "1.1.0",
			Classification: &classificationSupported,
		},
	}

	var versions = []core.MachineImageVersion{
		{
			ExpirableVersion: core.ExpirableVersion{
				Version:        "1.0.0",
				Classification: &classificationDeprecated,
			},
		},
		{
			ExpirableVersion: core.ExpirableVersion{
				Version:        "1.0.1",
				Classification: &classificationDeprecated,
			},
		},
		{
			ExpirableVersion: core.ExpirableVersion{
				Version:        "1.0.2",
				Classification: &classificationDeprecated,
			},
		},
		supportedVersion,
		deprecatedVersion,
		previewVersion,
	}

	var machineImages = []core.MachineImage{
		{
			Name: "coreos",
			Versions: []core.MachineImageVersion{
				{
					ExpirableVersion: core.ExpirableVersion{
						Version: "0.0.2",
					},
				},
				{
					ExpirableVersion: core.ExpirableVersion{
						Version: "0.0.3",
					},
				},
			},
		},
	}

	DescribeTable("#DetermineLatestMachineImageVersions",
		func(versions []core.MachineImage, expectation map[string]core.MachineImageVersion, expectError bool) {
			result, err := DetermineLatestMachineImageVersions(versions)
			if expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(result).To(Equal(expectation))
		},

		Entry("should return nil - empty machine image slice", nil, map[string]core.MachineImageVersion{}, false),
		Entry("should return nil - no valid image", []core.MachineImage{{Name: "coreos", Versions: []core.MachineImageVersion{{ExpirableVersion: core.ExpirableVersion{Version: "abc"}}}}}, nil, true),
		Entry("should determine latest expirable version", machineImages, map[string]core.MachineImageVersion{"coreos": {ExpirableVersion: core.ExpirableVersion{Version: "0.0.3"}}}, false),
	)

	DescribeTable("#DetermineLatestMachineImageVersion",
		func(versions []core.MachineImageVersion, filterPreviewVersions bool, expectation core.MachineImageVersion, expectError bool) {
			result, err := DetermineLatestMachineImageVersion(versions, filterPreviewVersions)
			if expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(result).To(Equal(expectation))
		},

		Entry("should determine latest expirable version - do not ignore preview version", versions, false, previewVersion, false),
		Entry("should determine latest expirable version - prefer older supported version over newer deprecated one (full list of versions)", versions, true, core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "1.1.0", Classification: &classificationSupported}}, false),
		Entry("should determine latest expirable version - prefer older supported version over newer deprecated one (latest non-deprecated version is earlier in the list)", []core.MachineImageVersion{supportedVersion, deprecatedVersion}, true, core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "1.1.0", Classification: &classificationSupported}}, false),
		Entry("should determine latest expirable version - prefer older supported version over newer deprecated one (latest deprecated version is earlier in the list)", []core.MachineImageVersion{deprecatedVersion, supportedVersion}, true, core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "1.1.0", Classification: &classificationSupported}}, false),
		Entry("should determine latest expirable version - select deprecated version when there is no supported one", []core.MachineImageVersion{previewVersion, deprecatedVersion}, true, core.MachineImageVersion{ExpirableVersion: core.ExpirableVersion{Version: "1.1.1", Classification: &classificationDeprecated}}, false),
		Entry("should return an error - only preview versions", []core.MachineImageVersion{previewVersion}, true, nil, true),
		Entry("should return an error - empty version slice", []core.MachineImageVersion{}, true, nil, true),
	)

	Describe("#GetRemovedVersions", func() {
		var (
			versions = []core.ExpirableVersion{
				{
					Version: "1.0.2",
				},
				{
					Version: "1.0.1",
				},
				{
					Version: "1.0.0",
				},
			}
		)
		It("should detect removed version", func() {
			diff := GetRemovedVersions(versions, versions[0:2])

			Expect(diff).To(HaveLen(1))
			Expect(diff["1.0.0"]).To(Equal(2))
		})

		It("should do nothing", func() {
			diff := GetRemovedVersions(versions, versions)

			Expect(diff).To(BeEmpty())
		})
	})

	Describe("#GetAddedVersions", func() {
		var (
			versions = []core.ExpirableVersion{
				{
					Version: "1.0.2",
				},
				{
					Version: "1.0.1",
				},
				{
					Version: "1.0.0",
				},
			}
		)
		It("should detected added versions", func() {
			diff := GetAddedVersions(versions[0:2], versions)

			Expect(diff).To(HaveLen(1))
			Expect(diff["1.0.0"]).To(Equal(2))
		})

		It("should do nothing", func() {
			diff := GetAddedVersions(versions, versions)

			Expect(diff).To(BeEmpty())
		})
	})

	Describe("#GetMachineImageDiff", func() {
		It("should return the diff between two machine image version slices", func() {
			versions1 := []core.MachineImage{
				{
					Name: "image-1",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "version-1"}},
						{ExpirableVersion: core.ExpirableVersion{Version: "version-2"}},
					},
				},
				{
					Name: "image-2",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "version-1"}},
						{ExpirableVersion: core.ExpirableVersion{Version: "version-2"}},
					},
				},
			}

			versions2 := []core.MachineImage{
				{
					Name: "image-2",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "version-2"}},
						{ExpirableVersion: core.ExpirableVersion{Version: "version-3"}},
					},
				},
				{
					Name: "image-3",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "version-1"}},
						{ExpirableVersion: core.ExpirableVersion{Version: "version-2"}},
					},
				},
			}

			removedImages, removedVersions, addedImages, addedVersions := GetMachineImageDiff(versions1, versions2)

			Expect(removedImages.UnsortedList()).To(ConsistOf("image-1"))
			Expect(removedVersions).To(BeEquivalentTo(
				map[string]sets.Set[string]{
					"image-1": sets.New("version-1", "version-2"),
					"image-2": sets.New("version-1"),
				},
			))
			Expect(addedImages.UnsortedList()).To(ConsistOf("image-3"))
			Expect(addedVersions).To(BeEquivalentTo(
				map[string]sets.Set[string]{
					"image-2": sets.New("version-3"),
					"image-3": sets.New("version-1", "version-2"),
				},
			))
		})

		It("should return the diff between an empty old and a filled new machine image slice", func() {
			versions2 := []core.MachineImage{
				{
					Name: "image-2",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "version-3"}},
					},
				},
				{
					Name: "image-3",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "version-1"}},
						{ExpirableVersion: core.ExpirableVersion{Version: "version-2"}},
					},
				},
			}

			removedImages, removedVersions, addedImages, addedVersions := GetMachineImageDiff(nil, versions2)

			Expect(removedImages.UnsortedList()).To(BeEmpty())
			Expect(removedVersions).To(BeEmpty())
			Expect(addedImages.UnsortedList()).To(ConsistOf("image-2", "image-3"))
			Expect(addedVersions).To(BeEquivalentTo(
				map[string]sets.Set[string]{
					"image-2": sets.New("version-3"),
					"image-3": sets.New("version-1", "version-2"),
				},
			))
		})

		It("should return the diff between a filled old and an empty new machine image slice", func() {
			versions1 := []core.MachineImage{
				{
					Name: "image-2",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "version-3"}},
					},
				},
				{
					Name: "image-3",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "version-1"}},
						{ExpirableVersion: core.ExpirableVersion{Version: "version-2"}},
					},
				},
			}

			removedImages, removedVersions, addedImages, addedVersions := GetMachineImageDiff(versions1, []core.MachineImage{})

			Expect(removedImages.UnsortedList()).To(ConsistOf("image-2", "image-3"))
			Expect(removedVersions).To(BeEquivalentTo(
				map[string]sets.Set[string]{
					"image-2": sets.New("version-3"),
					"image-3": sets.New("version-1", "version-2"),
				},
			))
			Expect(addedImages.UnsortedList()).To(BeEmpty())
			Expect(addedVersions).To(BeEmpty())
		})

		It("should return the diff between two empty machine image slices", func() {
			removedImages, removedVersions, addedImages, addedVersions := GetMachineImageDiff([]core.MachineImage{}, nil)

			Expect(removedImages.UnsortedList()).To(BeEmpty())
			Expect(removedVersions).To(BeEmpty())
			Expect(addedImages.UnsortedList()).To(BeEmpty())
			Expect(addedVersions).To(BeEmpty())
		})
	})

	Describe("#FilterVersionsWithClassification", func() {
		var (
			classification = core.ClassificationSupported
			versions       = []core.ExpirableVersion{
				{
					Version:        "1.0.2",
					Classification: &classification,
				},
				{
					Version:        "1.0.1",
					Classification: &classification,
				},
				{
					Version: "1.0.0",
				},
			}
		)

		It("should filter version", func() {
			filteredVersions := FilterVersionsWithClassification(versions, classification)

			Expect(filteredVersions).To(HaveLen(2))
			Expect(filteredVersions).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"Version":        Equal("1.0.2"),
				"Classification": Equal(&classification),
			}), MatchFields(IgnoreExtras, Fields{
				"Version":        Equal("1.0.1"),
				"Classification": Equal(&classification),
			})))
		})
	})

	Describe("#FindVersionsWithSameMajorMinor", func() {
		var (
			versions = []core.ExpirableVersion{
				{
					Version: "1.1.3",
				},
				{
					Version: "1.1.2",
				},
				{
					Version: "1.1.1",
				},
				{
					Version: "1.0.0",
				},
			}
		)
		It("should filter version", func() {
			currentSemVer, err := semver.NewVersion("1.1.3")
			Expect(err).ToNot(HaveOccurred())
			filteredVersions, _ := FindVersionsWithSameMajorMinor(versions, *currentSemVer)

			Expect(filteredVersions).To(HaveLen(2))
			Expect(filteredVersions).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"Version": Equal("1.1.2"),
			}), MatchFields(IgnoreExtras, Fields{
				"Version": Equal("1.1.1"),
			})))
		})
	})

	DescribeTable("#HasCapability",
		func(capabilityNames []string, requestedCapability string, expectedResult bool) {
			capabilities := []core.CapabilityDefinition{}
			for _, capabilityName := range capabilityNames {
				capabilities = append(capabilities, core.CapabilityDefinition{Name: capabilityName})
			}
			Expect(HasCapability(capabilities, requestedCapability)).To(Equal(expectedResult))
		},
		Entry("Should return false - no capabilities", nil, "foo", false),
		Entry("Should return true - one capability", []string{"foo"}, "foo", true),
		Entry("Should return true - many capabilities", []string{"foo", "bar"}, "foo", true),
	)

	DescribeTable("#ExtractArchitecturesFromImageFlavors",
		func(architecturesInSet1, architecturesInSet2, expectedResult []string) {
			var imageFlavors []core.MachineImageFlavor

			imageFlavors = append(imageFlavors, core.MachineImageFlavor{
				Capabilities: core.Capabilities{"architecture": architecturesInSet1},
			})

			imageFlavors = append(imageFlavors, core.MachineImageFlavor{
				Capabilities: core.Capabilities{"architecture": architecturesInSet2},
			})

			Expect(ExtractArchitecturesFromImageFlavors(imageFlavors)).To(ConsistOf(expectedResult))
		},
		Entry("Should return no values", nil, nil, []string{}),
		Entry("Should return architecture in sets (sets partially filled)", []string{"amd64", "arm64"}, []string{"ia-64"}, []string{"amd64", "arm64", "ia-64"}),
		Entry("Should return architecture in sets (all sets filled)", []string{"amd64", "arm64"}, nil, []string{"amd64", "arm64"}),
	)

	DescribeTable("#CapabilityDefinitionsToCapabilities",
		func(capabilityNames ...string) {
			var (
				capabilities = make([]core.CapabilityDefinition, 0, len(capabilityNames))
				values       = core.CapabilityValues{"value-1", "value-2"}
			)

			for _, capabilityName := range capabilityNames {
				capabilities = append(capabilities, core.CapabilityDefinition{Name: capabilityName, Values: values})
			}

			capabilitiesMap := CapabilityDefinitionsToCapabilities(capabilities)

			if len(capabilityNames) == 0 {
				Expect(capabilitiesMap).To(BeEmpty())
			} else {
				for _, capability := range capabilities {
					Expect(capabilitiesMap).To(HaveKeyWithValue(capability.Name, values), "capability "+capability.Name+" with values "+strings.Join(values, ",")+" not found")
				}
			}
		},
		Entry("without capabilities", nil),
		Entry("with capabilities", "architecture", "network"),
	)

	Describe("#GetCapabilitiesWithAppliedDefaults", func() {
		It("should apply default values when capabilities are nil", func() {
			var capabilities core.Capabilities
			capabilityDefinitions := []core.CapabilityDefinition{
				{Name: "capability1", Values: []string{"value1", "value2"}},
				{Name: "architecture", Values: []string{"amd64"}},
			}

			result := GetCapabilitiesWithAppliedDefaults(capabilities, capabilityDefinitions)

			Expect(result).To(Equal(core.Capabilities{
				"capability1":  []string{"value1", "value2"},
				"architecture": []string{"amd64"},
			}))
		})

		It("should retain existing values and apply defaults for missing capabilities", func() {
			capabilities := core.Capabilities{
				"capability1": []string{"value1"},
			}
			capabilityDefinitions := []core.CapabilityDefinition{
				{Name: "capability1", Values: []string{"value1", "value2"}},
				{Name: "architecture", Values: []string{"amd64"}},
			}

			result := GetCapabilitiesWithAppliedDefaults(capabilities, capabilityDefinitions)

			Expect(result).To(Equal(core.Capabilities{
				"capability1":  []string{"value1"},
				"architecture": []string{"amd64"},
			}))
		})
	})

	Describe("#GetImageFlavorsWithAppliedDefaults", func() {
		It("should apply defaults when capabilityFlavors are empty", func() {
			var imageFlavors []core.MachineImageFlavor
			capabilityDefinitions := []core.CapabilityDefinition{
				{Name: "capability1", Values: []string{"value1", "value2"}},
				{Name: "architecture", Values: []string{"amd64"}},
			}

			result := GetImageFlavorsWithAppliedDefaults(imageFlavors, capabilityDefinitions)

			Expect(result).To(HaveLen(1))
			Expect(result[0].Capabilities).To(Equal(core.Capabilities{
				"capability1":  []string{"value1", "value2"},
				"architecture": []string{"amd64"},
			}))
		})

		It("should retain existing values and apply defaults for missing capabilities in sets", func() {
			imageFlavors := []core.MachineImageFlavor{
				{Capabilities: core.Capabilities{"capability1": []string{"value1"}}},
				{Capabilities: core.Capabilities{"architecture": []string{"arm64"}}},
			}
			capabilityDefinitions := []core.CapabilityDefinition{
				{Name: "capability1", Values: []string{"value1", "value2"}},
				{Name: "architecture", Values: []string{"amd64", "arm64"}},
			}

			result := GetImageFlavorsWithAppliedDefaults(imageFlavors, capabilityDefinitions)

			Expect(result).To(HaveLen(2))
			Expect(result[0].Capabilities).To(Equal(core.Capabilities{
				"capability1":  []string{"value1"},
				"architecture": []string{"amd64", "arm64"},
			}))
			Expect(result[1].Capabilities).To(Equal(core.Capabilities{
				"capability1":  []string{"value1", "value2"},
				"architecture": []string{"arm64"},
			}))
		})
	})

	Describe("#AreCapabilitiesEqual", func() {

		It("should return true for equal capabilities", func() {
			a := core.Capabilities{
				"key1": {"value1"},
				"key2": {"valueA", "valueB"},
			}
			b := core.Capabilities{
				"key1": {"value1"},
				"key2": {"valueA", "valueB"},
			}

			result := AreCapabilitiesEqual(a, b)
			Expect(result).To(BeTrue())
		})

		It("should return false for capabilities with different keys", func() {
			a := core.Capabilities{"key1": {"value1"}}
			b := core.Capabilities{"key2": {"value1"}}

			result := AreCapabilitiesEqual(a, b)
			Expect(result).To(BeFalse())
		})

		It("should return false for capabilities with different values", func() {
			a := core.Capabilities{
				"key1": {"value1"},
				"key2": {"valueA", "valueB"},
			}
			b := core.Capabilities{
				"key1": {"value2"},
				"key2": {"valueA", "valueB"},
			}

			result := AreCapabilitiesEqual(a, b)
			Expect(result).To(BeFalse())
		})
	})
})
