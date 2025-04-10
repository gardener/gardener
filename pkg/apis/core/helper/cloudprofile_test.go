// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/helper"
)

var _ = Describe("Helper", func() {
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
		classification := core.ClassificationDeprecated
		var (
			versions = []core.ExpirableVersion{
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

	Describe("#SyncArchitectureCapabilityFields", func() {
		var (
			cloudProfileSpecNew core.CloudProfileSpec
			cloudProfileSpecOld core.CloudProfileSpec
		)

		BeforeEach(func() {
			cloudProfileSpecNew = core.CloudProfileSpec{
				MachineImages: []core.MachineImage{
					{Versions: []core.MachineImageVersion{{}}},
				},
				MachineTypes: []core.MachineType{
					{},
				},
			}
			cloudProfileSpecOld = cloudProfileSpecNew
		})

		Describe("Initial migration", func() {
			BeforeEach(func() {
				cloudProfileSpecNew.Capabilities = []core.Capability{
					{Name: "architecture", Values: []string{"arm64", "amd64", "custom"}},
				}
			})

			It("It should do nothing for empty architectures and empty capabilities", func() {
				cloudProfileSpecNewBefore := cloudProfileSpecNew
				// With the update, the old fields are unset:
				cloudProfileSpecOld.MachineImages[0].Versions[0].Architectures = []string{"amd64"}
				cloudProfileSpecOld.MachineTypes[0].Architecture = ptr.To("amd64")

				SyncArchitectureCapabilityFields(cloudProfileSpecNew, cloudProfileSpecOld)

				Expect(cloudProfileSpecNew).To(Equal(cloudProfileSpecNewBefore))
			})

			It("It should correctly handle split-up machine image version capability architectures", func() {
				cloudProfileSpecNew.MachineImages[0].Versions[0].CapabilitySets = []core.CapabilitySet{
					{Capabilities: core.Capabilities{"architecture": []string{"custom"}}},
					{Capabilities: core.Capabilities{"architecture": []string{"amd64"}}},
					{Capabilities: core.Capabilities{"architecture": []string{"arm64"}}},
				}

				SyncArchitectureCapabilityFields(cloudProfileSpecNew, cloudProfileSpecOld)

				Expect(cloudProfileSpecNew.MachineImages[0].Versions[0].Architectures).To(ConsistOf("amd64", "arm64", "custom"))
			})

			It("It should sync filled architecture fields to empty capabilities", func() {
				cloudProfileSpecNew.MachineImages[0].Versions[0].Architectures = []string{"amd64", "arm64"}
				cloudProfileSpecNew.MachineTypes[0].Architecture = ptr.To("amd64")

				SyncArchitectureCapabilityFields(cloudProfileSpecNew, cloudProfileSpecOld)

				Expect(cloudProfileSpecNew.MachineImages[0].Versions[0].Architectures).To(Equal([]string{"amd64", "arm64"}))
				Expect(cloudProfileSpecNew.MachineImages[0].Versions[0].CapabilitySets[0].Capabilities["architecture"]).To(BeEquivalentTo([]string{"amd64"}))
				Expect(cloudProfileSpecNew.MachineImages[0].Versions[0].CapabilitySets[1].Capabilities["architecture"]).To(BeEquivalentTo([]string{"arm64"}))
				Expect(cloudProfileSpecNew.MachineTypes[0].Architecture).To(Equal(ptr.To("amd64")))
				Expect(cloudProfileSpecNew.MachineTypes[0].Capabilities["architecture"]).To(BeEquivalentTo([]string{"amd64"}))
			})
		})
	})
})
