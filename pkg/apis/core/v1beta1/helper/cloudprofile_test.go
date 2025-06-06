// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

var _ = Describe("CloudProfile Helper", func() {
	var (
		trueVar                 = true
		expirationDateInThePast = metav1.Time{Time: time.Now().AddDate(0, 0, -1)}
	)

	Describe("#CurrentLifecycleClassification", func() {
		It("version is implicitly supported", func() {
			classification := CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				Version: "1.28.0",
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationSupported))
		})

		It("version is explicitly supported", func() {
			classification := CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				Version:        "1.28.0",
				Classification: ptr.To(gardencorev1beta1.ClassificationSupported),
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationSupported))
		})

		It("version is in preview stage", func() {
			classification := CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				Version:        "1.28.0",
				Classification: ptr.To(gardencorev1beta1.ClassificationPreview),
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationPreview))
		})

		It("version is deprecated ", func() {
			classification := CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				Version:        "1.28.0",
				Classification: ptr.To(gardencorev1beta1.ClassificationDeprecated),
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationDeprecated))
		})

		It("supported version will expire in the future", func() {
			classification := CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				Version:        "1.28.0",
				Classification: ptr.To(gardencorev1beta1.ClassificationSupported),
				ExpirationDate: ptr.To(metav1.NewTime(time.Now().Add(2 * time.Hour))),
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationSupported))
		})

		It("supported version has already expired", func() {
			classification := CurrentLifecycleClassification(gardencorev1beta1.ExpirableVersion{
				Version:        "1.28.0",
				Classification: ptr.To(gardencorev1beta1.ClassificationSupported),
				ExpirationDate: ptr.To(metav1.NewTime(time.Now().Add(-2 * time.Hour))),
			})
			Expect(classification).To(Equal(gardencorev1beta1.ClassificationExpired))
		})
	})

	Describe("#FindMachineImageVersion", func() {
		var machineImages []gardencorev1beta1.MachineImage

		BeforeEach(func() {
			machineImages = []gardencorev1beta1.MachineImage{
				{
					Name: "coreos",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "0.0.2",
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "0.0.3",
							},
						},
					},
				},
			}
		})

		It("should find the machine image version when it exists", func() {
			expected := gardencorev1beta1.MachineImageVersion{
				ExpirableVersion: gardencorev1beta1.ExpirableVersion{
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
			Expect(actual).To(Equal(gardencorev1beta1.MachineImageVersion{}))
		})

		It("should return false when machine image version with the given version does not exist", func() {
			actual, ok := FindMachineImageVersion(machineImages, "coreos", "0.0.4")
			Expect(ok).To(BeFalse())
			Expect(actual).To(Equal(gardencorev1beta1.MachineImageVersion{}))
		})
	})

	Describe("#ShootMachineImageVersionExists", func() {
		var (
			constraint        gardencorev1beta1.MachineImage
			shootMachineImage gardencorev1beta1.ShootMachineImage
		)

		BeforeEach(func() {
			constraint = gardencorev1beta1.MachineImage{
				Name: "coreos",
				Versions: []gardencorev1beta1.MachineImageVersion{
					{
						ExpirableVersion: gardencorev1beta1.ExpirableVersion{
							Version: "0.0.2",
						},
					},
					{
						ExpirableVersion: gardencorev1beta1.ExpirableVersion{
							Version: "0.0.3",
						},
					},
				},
			}

			shootMachineImage = gardencorev1beta1.ShootMachineImage{
				Name:    "coreos",
				Version: ptr.To("0.0.2"),
			}
		})

		It("should determine that the version exists", func() {
			exists, index := ShootMachineImageVersionExists(constraint, shootMachineImage)
			Expect(exists).To(Equal(trueVar))
			Expect(index).To(Equal(0))
		})

		It("should determine that the version does not exist", func() {
			shootMachineImage.Name = "xy"
			exists, _ := ShootMachineImageVersionExists(constraint, shootMachineImage)
			Expect(exists).To(BeFalse())
		})

		It("should determine that the version does not exist", func() {
			shootMachineImage.Version = ptr.To("0.0.4")
			exists, _ := ShootMachineImageVersionExists(constraint, shootMachineImage)
			Expect(exists).To(BeFalse())
		})
	})

	DescribeTable("#FindMachineTypeByName",
		func(machines []gardencorev1beta1.MachineType, name string, expectedMachine *gardencorev1beta1.MachineType) {
			Expect(FindMachineTypeByName(machines, name)).To(Equal(expectedMachine))
		},

		Entry("no workers", nil, "", nil),
		Entry("worker not found", []gardencorev1beta1.MachineType{{Name: "foo"}}, "bar", nil),
		Entry("worker found", []gardencorev1beta1.MachineType{{Name: "foo"}}, "foo", &gardencorev1beta1.MachineType{Name: "foo"}),
	)

	var previewClassification = gardencorev1beta1.ClassificationPreview
	var deprecatedClassification = gardencorev1beta1.ClassificationDeprecated
	var supportedClassification = gardencorev1beta1.ClassificationSupported

	DescribeTable("#GetOverallLatestVersionForAutoUpdate",
		func(versions []gardencorev1beta1.ExpirableVersion, currentVersion string, foundVersion bool, expectedVersion string, expectError bool) {
			qualifyingVersionFound, latestVersion, err := GetOverallLatestVersionForAutoUpdate(versions, currentVersion)
			if expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(qualifyingVersionFound).To(Equal(foundVersion))
			Expect(latestVersion).To(Equal(expectedVersion))
		},
		Entry("Get latest version",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version: "1.17.1",
				},
				{
					Version: "1.15.0",
				},
				{
					Version: "1.14.3",
				},
				{
					Version: "1.13.1",
				},
			},
			"1.14.3",
			true,
			"1.17.1",
			false,
		),
		Entry("Get latest version across major versions",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version: "3.0.1",
				},
				{
					Version:        "2.1.1",
					Classification: &deprecatedClassification,
				},
				{
					Version:        "2.0.0",
					Classification: &supportedClassification,
				},
				{
					Version: "0.4.1",
				},
			},
			"0.4.1",
			true,
			"3.0.1",
			false,
		),
		Entry("Get latest version across major versions, preferring lower supported version",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version:        "3.0.1",
					Classification: &deprecatedClassification,
				},
				{
					Version:        "2.1.1",
					Classification: &deprecatedClassification,
				},
				{
					Version:        "2.0.0",
					Classification: &supportedClassification,
				},
				{
					Version: "0.4.1",
				},
			},
			"0.4.1",
			true,
			"2.0.0",
			false,
		),
		Entry("Expect no higher version than the current version to be found, as already on the latest version",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version: "1.17.1",
				},
				{
					Version: "1.15.0",
				},
				{
					Version: "1.14.3",
				},
				{
					Version: "1.13.1",
				},
			},
			"1.17.1",
			false,
			"",
			false,
		),
		Entry("Expect to first update to the latest patch version of the same minor before updating to the overall latest version",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version: "1.17.1",
				},
				{
					Version: "1.15.3",
				},
				{
					Version: "1.15.0",
				},
			},
			"1.15.0",
			true,
			"1.15.3",
			false,
		),
		Entry("Expect no qualifying version to be found - machine image has only versions in preview and expired versions",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version:        "1.17.1",
					Classification: &previewClassification,
				},
				{
					Version:        "1.15.0",
					Classification: &previewClassification,
				},
				{
					Version:        "1.14.3",
					ExpirationDate: &expirationDateInThePast,
				},
				{
					Version:        "1.13.1",
					ExpirationDate: &expirationDateInThePast,
				},
			},
			"1.13.1",
			false,
			"",
			false,
		),
		Entry("Expect older but supported version to be preferred over newer but deprecated one",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version:        "1.17.1",
					Classification: &deprecatedClassification,
				},
				{
					Version:        "1.16.1",
					Classification: &supportedClassification,
				},
				{
					Version:        "1.15.0",
					Classification: &previewClassification,
				},
				{
					Version:        "1.14.3",
					ExpirationDate: &expirationDateInThePast,
				},
				{
					Version:        "1.13.1",
					ExpirationDate: &expirationDateInThePast,
				},
			},
			"1.13.1",
			true,
			"1.16.1",
			false,
		),
		Entry("Expect latest deprecated version to be selected when there is no supported version",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version:        "1.17.3",
					Classification: &previewClassification,
				},
				{
					Version:        "1.17.2",
					ExpirationDate: &expirationDateInThePast,
				},
				{
					Version:        "1.17.1",
					Classification: &deprecatedClassification,
				},
				{
					Version:        "1.16.1",
					Classification: &deprecatedClassification,
				},
				{
					Version:        "1.15.0",
					Classification: &previewClassification,
				},
				{
					Version:        "1.14.3",
					ExpirationDate: &expirationDateInThePast,
				},
			},
			"1.14.3",
			true,
			"1.17.1",
			false,
		),
	)

	DescribeTable("#GetLatestQualifyingVersion",
		func(original []gardencorev1beta1.ExpirableVersion, expectVersionToBeFound bool, expected *gardencorev1beta1.ExpirableVersion, expectError bool) {
			qualifyingVersionFound, latestVersion, err := GetLatestQualifyingVersion(original, nil)
			if expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(qualifyingVersionFound).To(Equal(expectVersionToBeFound))
			Expect(latestVersion).To(Equal(expected))
		},
		Entry("Get latest non-preview version",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version:        "1.17.2",
					Classification: &previewClassification,
				},
				{
					Version: "1.17.1",
				},
				{
					Version: "1.15.0",
				},
				{
					Version: "1.14.3",
				},
				{
					Version: "1.13.1",
				},
			},
			true,
			&gardencorev1beta1.ExpirableVersion{
				Version: "1.17.1",
			},
			false,
		),
		Entry("Expect no qualifying version to be found - no latest version could be found",
			[]gardencorev1beta1.ExpirableVersion{},
			false,
			nil,
			false,
		),
		Entry("Expect error, because contains invalid semVer",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version: "1.213123xx",
				},
			},
			false,
			nil,
			true,
		),
	)

	DescribeTable("#GetQualifyingVersionForNextHigher",
		func(original []gardencorev1beta1.ExpirableVersion, currentVersion string, getNextHigherMinor bool, expectVersionToBeFound bool, expected *string, expectedNextMinorOrMajorVersion uint64, expectError bool) {
			var (
				majorMinor    GetMajorOrMinor
				filterSmaller VersionPredicate
			)

			currentSemVerVersion := semver.MustParse(currentVersion)

			// setup filter for smaller minor or smaller major
			if getNextHigherMinor {
				majorMinor = func(v semver.Version) uint64 { return v.Minor() }
				filterSmaller = FilterEqualAndSmallerMinorVersion(*currentSemVerVersion)
			} else {
				majorMinor = func(v semver.Version) uint64 { return v.Major() }
				filterSmaller = FilterEqualAndSmallerMajorVersion(*currentSemVerVersion)
			}

			foundVersion, qualifyingVersion, nextMinorOrMajorVersion, err := GetQualifyingVersionForNextHigher(original, majorMinor, currentSemVerVersion, filterSmaller)
			if expectError {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(nextMinorOrMajorVersion).To(Equal(expectedNextMinorOrMajorVersion))
			Expect(err).ToNot(HaveOccurred())
			Expect(foundVersion).To(Equal(expectVersionToBeFound))
			if foundVersion {
				Expect(qualifyingVersion.Version).To(Equal(*expected))
			}
		},
		Entry("Get latest non-preview version for next higher minor version",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version:        "1.3.2",
					Classification: &previewClassification,
				},
				{
					Version:        "1.3.2",
					ExpirationDate: &expirationDateInThePast,
				},
				{
					Version:        "1.3.1",
					ExpirationDate: &expirationDateInThePast,
				},
				{
					Version: "1.1.1",
				},
				{
					Version: "1.0.0",
				},
			},
			"1.1.0",
			true, // target minor
			true,
			ptr.To("1.3.2"),
			uint64(3), // next minor version to be found
			false,
		),
		Entry("Get latest non-preview version for next higher major version",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version:        "4.4.2",
					Classification: &previewClassification,
				},
				{
					Version:        "4.3.2",
					ExpirationDate: &expirationDateInThePast,
				},
				{
					Version:        "4.3.1",
					ExpirationDate: &expirationDateInThePast,
				},
				{
					Version: "1.1.0",
				},
				{
					Version: "1.0.0",
				},
			},
			"1.1.0",
			false, // target major
			true,
			ptr.To("4.3.2"),
			uint64(4), // next major version to be found
			false,
		),
		Entry("Skip next higher minor version if contains no qualifying version",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version: "1.4.2",
				},
				{
					Version:        "1.3.2",
					Classification: &previewClassification,
				},
				{
					Version: "1.1.1",
				},
				{
					Version: "1.0.0",
				},
			},
			"1.1.0",
			true, // target minor
			true,
			ptr.To("1.4.2"),
			uint64(3), // next minor version to be found
			false,
		),
		Entry("Skip next higher major version if contains no qualifying version",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version: "4.4.2",
				},
				{
					Version:        "3.3.2",
					Classification: &previewClassification,
				},
				{
					Version: "1.1.1",
				},
				{
					Version: "1.0.0",
				},
			},
			"1.1.0",
			false, // target major
			true,
			ptr.To("4.4.2"),
			uint64(3), // next major version to be found
			false,
		),
		Entry("Expect no version to be found: already on highest version in major",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version: "2.0.0",
				},
				{
					Version:        "1.3.2",
					Classification: &previewClassification,
				},
				{
					Version: "1.1.1",
				},
				{
					Version: "1.0.0",
				},
			},
			"1.1.0",
			true, // target minor
			false,
			nil,
			uint64(3), // next minor version to be found
			false,
		),
		Entry("Expect no version to be found: already on overall highest version",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version: "2.0.0",
				},
			},
			"2.0.0",
			false, // target major
			false,
			nil,
			uint64(0), // next minor version to be found
			false,
		),
		Entry("Expect no qualifying version to be found - no latest version could be found",
			[]gardencorev1beta1.ExpirableVersion{},
			"1.1.0",
			true, // target minor
			false,
			nil,
			uint64(0),
			false,
		),
		Entry("Expect error, because contains invalid semVer",
			[]gardencorev1beta1.ExpirableVersion{
				{
					Version: "1.213123xx",
				},
			},
			"1.1.0",
			false,
			false,
			nil,
			uint64(1),
			true,
		),
	)

	Describe("#Expirable Version Helper", func() {
		classificationPreview := gardencorev1beta1.ClassificationPreview

		DescribeTable("#GetLatestVersionForPatchAutoUpdate",
			func(currentVersion string, cloudProfileVersions []gardencorev1beta1.ExpirableVersion, expectedVersion string, qualifyingVersionFound bool) {
				ok, newVersion, err := GetLatestVersionForPatchAutoUpdate(cloudProfileVersions, currentVersion)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(Equal(qualifyingVersionFound))
				Expect(newVersion).To(Equal(expectedVersion))
			},
			Entry("Do not consider preview versions for patch update.",
				"1.12.2",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
					{
						Version:        "1.12.9",
						Classification: &previewClassification,
					},
					{
						Version:        "1.12.4",
						Classification: &previewClassification,
					},
					// latest qualifying version for updating version 1.12.2
					{Version: "1.12.3"},
					{Version: "1.12.2"},
				},
				"1.12.3",
				true,
			),
			Entry("Do not consider expired versions for patch update.",
				"1.12.2",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
					{
						Version:        "1.12.9",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.12.4",
						ExpirationDate: &expirationDateInThePast,
					},
					// latest qualifying version for updating version 1.12.2
					{Version: "1.12.3"},
					{Version: "1.12.2"},
				},
				"1.12.3",
				true,
			),
			Entry("Should not find qualifying version - no higher version available that is not expired or in preview.",
				"1.12.2",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
					{
						Version:        "1.12.9",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.12.4",
						Classification: &previewClassification,
					},
					{Version: "1.12.2"},
				},
				"",
				false,
			),
			Entry("Should not find qualifying version - is already highest version of minor.",
				"1.12.2",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
					{Version: "1.12.2"},
					{Version: "1.12.1"},
				},
				"",
				false,
			),
			Entry("Should not find qualifying version - is already on latest version of latest minor.",
				"1.15.1",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
					{Version: "1.12.2"},
				},
				"",
				false,
			),
		)

		DescribeTable("#GetLatestVersionForMinorAutoUpdate",
			func(currentVersion string, cloudProfileVersions []gardencorev1beta1.ExpirableVersion, expectedVersion string, qualifyingVersionFound bool) {
				foundVersion, newVersion, err := GetLatestVersionForMinorAutoUpdate(cloudProfileVersions, currentVersion)
				Expect(err).ToNot(HaveOccurred())
				Expect(foundVersion).To(Equal(qualifyingVersionFound))
				Expect(newVersion).To(Equal(expectedVersion))
			},
			Entry("Should find qualifying version - the latest version for the major.",
				"1.12.2",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "2.0.0"},
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
					{Version: "1.12.2"},
				},
				"1.15.1",
				true,
			),
			Entry("Should find qualifying version - the latest version for the major.",
				"0.2.3",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "2.0.0"},
					{Version: "1.15.1"},
					{Version: "0.4.1"},
					{Version: "0.4.0"},
					{Version: "0.2.3"},
				},
				"0.4.1",
				true,
			),
			Entry("Should find qualifying version - do not consider preview versions for auto updates.",
				"1.12.2",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "2.0.0"},
					{
						Version:        "1.15.2",
						Classification: &previewClassification,
					},
					// latest qualifying version for updating version 1.12.2 to the latest version for the major
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
				},
				"1.15.1",
				true,
			),
			Entry("Should find qualifying version - always first update to latest patch of minor.",
				"1.12.2",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "2.0.0"},
					{
						Version:        "1.15.2",
						Classification: &previewClassification,
					},
					// latest qualifying version for updating version 1.12.2 to the latest version for the major
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
					{Version: "1.12.3"},
					{Version: "1.12.2"},
				},
				"1.12.3",
				true,
			),
			Entry("Should find qualifying version - do not consider expired versions for auto updates.",
				"1.1.2",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "2.0.0"},
					{
						Version:        "1.12.9",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.12.4",
						ExpirationDate: &expirationDateInThePast,
					},
					// latest qualifying version for updating version 1.1.2
					{Version: "1.10.3"},
					{Version: "1.10.2"},
					{Version: "1.1.2"},
					{Version: "1.0.2"},
				},
				"1.10.3",
				true,
			),
			Entry("Should not find qualifying version - no higher version available that is not expired or in preview.",
				"1.12.2",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version:        "1.15.1",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.15.0",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.14.4",
						Classification: &previewClassification,
					},
					{Version: "1.12.2"},
				},
				"",
				false,
			),
			Entry("Should not find qualifying version - is already highest version of major.",
				"1.15.1",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "2.0.0"},
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
					{Version: "1.12.2"},
					{Version: "1.12.1"},
				},
				"",
				false,
			),
			Entry("Should not find qualifying version - current version is higher than any given version in major.",
				"1.17.1",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "2.0.0"},
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
					{Version: "1.12.2"},
					{Version: "1.12.1"},
				},
				"",
				false,
			),
		)

		DescribeTable("#GetVersionForForcefulUpdateToConsecutiveMinor",
			func(currentVersion string, cloudProfileVersions []gardencorev1beta1.ExpirableVersion, expectedVersion string, qualifyingVersionFound bool) {
				ok, newVersion, err := GetVersionForForcefulUpdateToConsecutiveMinor(cloudProfileVersions, currentVersion)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(Equal(qualifyingVersionFound))
				Expect(newVersion).To(Equal(expectedVersion))
			},
			Entry("Do not consider preview versions of the consecutive minor version.",
				"1.11.3",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{
						Version:        "1.12.9",
						Classification: &previewClassification,
					},
					{
						Version:        "1.12.4",
						Classification: &previewClassification,
					},
					// latest qualifying version for minor version update for version 1.11.3
					{Version: "1.12.3"},
					{Version: "1.12.2"},
					{Version: "1.11.3"},
				},
				"1.12.3",
				true,
			),
			Entry("Should find qualifying version - latest non-expired version of the consecutive minor version.",
				"1.11.3",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{
						Version:        "1.12.9",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.12.4",
						ExpirationDate: &expirationDateInThePast,
					},
					// latest qualifying version for updating version 1.11.3
					{Version: "1.12.3"},
					{Version: "1.12.2"},
					{Version: "1.11.3"},
					{Version: "1.10.1"},
					{Version: "1.9.0"},
				},
				"1.12.3",
				true,
			),
			// check that multiple consecutive minor versions are possible
			Entry("Should find qualifying version if there are only expired versions available in the consecutive minor version - pick latest expired version of that minor.",
				"1.11.3",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					// latest qualifying version for updating version 1.11.3
					{
						Version:        "1.12.9",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.12.4",
						ExpirationDate: &expirationDateInThePast,
					},
					{Version: "1.11.3"},
				},
				"1.12.9",
				true,
			),
			Entry("Should not find qualifying version - there is no consecutive minor version available.",
				"1.10.3",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "2.0.0"},
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{
						Version:        "1.12.9",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.12.4",
						ExpirationDate: &expirationDateInThePast,
					},
					{Version: "1.12.3"},
					{Version: "1.12.2"},
					{Version: "1.10.3"},
				},
				"",
				false,
			),
			Entry("Should not find qualifying version - already on latest minor version.",
				"1.15.1",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
					{Version: "1.12.2"},
				},
				"",
				false,
			),
			Entry("Should not find qualifying version - is already on latest version of latest minor version.",
				"1.15.1",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
					{Version: "1.12.2"},
				},
				"",
				false,
			),
		)

		DescribeTable("#GetVersionForForcefulUpdateToNextHigherMinor",
			func(currentVersion string, cloudProfileVersions []gardencorev1beta1.ExpirableVersion, expectedVersion string, qualifyingVersionFound bool) {
				ok, newVersion, err := GetVersionForForcefulUpdateToNextHigherMinor(cloudProfileVersions, currentVersion)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(Equal(qualifyingVersionFound))
				Expect(newVersion).To(Equal(expectedVersion))
			},
			Entry("Should find qualifying version - but do not consider preview versions of the next minor version.",
				"1.11.3",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{
						Version:        "1.13.9",
						Classification: &previewClassification,
					},
					{
						Version:        "1.13.4",
						Classification: &previewClassification,
					},
					// latest qualifying version for minor version update for version 1.11.3
					{Version: "1.13.3"},
					{Version: "1.13.2"},
					{Version: "1.11.3"},
				},
				"1.13.3",
				true,
			),
			Entry("Should find qualifying version - latest non-expired version of the next minor version.",
				"1.11.3",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{
						Version:        "1.12.9",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.12.4",
						ExpirationDate: &expirationDateInThePast,
					},
					// latest qualifying version for updating version 1.11.3
					{Version: "1.12.3"},
					{Version: "1.12.2"},
					{Version: "1.11.3"},
					{Version: "1.10.1"},
					{Version: "1.9.0"},
				},
				"1.12.3",
				true,
			),
			Entry("Should find qualifying version if the latest version in next minor is expired - pick latest non-expired version of that minor.",
				"1.11.3",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					// latest qualifying version for updating version 1.11.3
					{
						Version:        "1.13.9",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version: "1.13.4",
					},
					{Version: "1.11.3"},
				},
				"1.13.4",
				true,
			),
			// check that multiple consecutive minor versions are possible
			Entry("Should find qualifying version if there are only expired versions available in the next minor version - pick latest expired version of that minor.",
				"1.11.3",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					// latest qualifying version for updating version 1.11.3
					{
						Version:        "1.13.9",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.13.4",
						ExpirationDate: &expirationDateInThePast,
					},
					{Version: "1.11.3"},
				},
				"1.13.9",
				true,
			),
			Entry("Should find qualifying version - there is a next higher minor version available.",
				"1.10.3",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{
						Version:        "1.12.9",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1.12.4",
						ExpirationDate: &expirationDateInThePast,
					},
					{Version: "1.12.3"},
					{Version: "1.12.2"},
					{Version: "1.10.3"},
				},
				"1.12.3",
				true,
			),
			Entry("Should find qualifying version - but skip over next higher minor as it does not contain qualifying versions.",
				"1.10.3",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{
						Version:        "1.12.9",
						Classification: &classificationPreview,
					},
					{
						Version:        "1.12.4",
						Classification: &classificationPreview,
					},
					{Version: "1.10.3"},
				},
				"1.15.1",
				true,
			),
			Entry("Should not find qualifying version - already on latest minor version.",
				"1.17.1",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "2.0.0"},
					{Version: "1.17.1"},
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
					{Version: "1.12.2"},
				},
				"",
				false,
			),
			Entry("Should not find qualifying version - is already on latest version of latest minor version.",
				"1.15.1",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
					{Version: "1.12.2"},
				},
				"",
				false,
			),
		)

		DescribeTable("#GetVersionForForcefulUpdateToNextHigherMajor",
			func(currentVersion string, cloudProfileVersions []gardencorev1beta1.ExpirableVersion, expectedVersion string, qualifyingVersionFound bool) {
				ok, newVersion, err := GetVersionForForcefulUpdateToNextHigherMajor(cloudProfileVersions, currentVersion)
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(Equal(qualifyingVersionFound))
				Expect(newVersion).To(Equal(expectedVersion))
			},
			Entry("Should find qualifying version - but do not consider preview versions of the next major version.",
				"534.6.3",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1096.0.0"},
					// latest qualifying version for minor version update for version 1.11.3
					{Version: "1034.1.1"},
					{Version: "1034.0.0"},
					{
						Version:        "1034.0.9",
						Classification: &previewClassification,
					},
					{
						Version:        "1034.1.4",
						Classification: &previewClassification,
					},
					{Version: "534.6.3"},
					{Version: "534.5.0"},
					{Version: "1.11.3"},
				},
				"1034.1.1",
				true,
			),
			Entry("Should find qualifying version - latest non-expired version of the next major version.",
				"534.0.0",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1096.0.0"},
					{Version: "1034.5.1"},
					{Version: "1034.5.0"},
					{Version: "1034.2.0"},
					{
						Version:        "1034.1.0",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1034.0.0",
						ExpirationDate: &expirationDateInThePast,
					},
					{Version: "534.0.0"},
				},
				"1034.5.1",
				true,
			),
			Entry("Should find qualifying version if the latest version in next major is expired - pick latest non-expired version of that major.",
				"534.0.0",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1096.0.1"},
					{Version: "1096.0.0"},
					// latest qualifying version for updating version 1.11.3
					{
						Version:        "1034.1.1",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version: "1034.1.0",
					},
					{
						Version:        "1034.0.1",
						ExpirationDate: &expirationDateInThePast,
					},
					{Version: "534.0.0"},
				},
				"1034.1.0",
				true,
			),
			Entry("Should find qualifying version if there are only expired versions available in the next major version - pick latest expired version of that major.",
				"534.0.0",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1096.0.1"},
					{Version: "1096.0.0"},
					// latest qualifying version for updating version 1.11.3
					{
						Version:        "1034.1.1",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1034.1.0",
						ExpirationDate: &expirationDateInThePast,
					},
					{
						Version:        "1034.0.1",
						ExpirationDate: &expirationDateInThePast,
					},
					{Version: "534.0.0"},
				},
				"1034.1.1",
				true,
			),
			Entry("Should find qualifying version - skip over next higher major as it contains no qualifying version.",
				"534.0.0",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "1096.1.0"},
					{Version: "1096.0.0"},
					{
						Version:        "1034.1.0",
						Classification: &previewClassification,
					},
					{
						Version:        "1034.0.0",
						Classification: &previewClassification,
					},
					{Version: "534.0.0"},
				},
				"1096.1.0",
				true,
			),
			Entry("Should not find qualifying version - already on latest overall version.",
				"2.1.0",
				[]gardencorev1beta1.ExpirableVersion{
					{Version: "2.1.0"},
					{Version: "2.0.0"},
					{Version: "1.17.1"},
					{Version: "1.15.1"},
					{Version: "1.15.0"},
					{Version: "1.14.4"},
					{Version: "1.12.2"},
				},
				"",
				false,
			),
		)

		DescribeTable("Test version filter predicates",
			func(predicate VersionPredicate, version *semver.Version, expirableVersion gardencorev1beta1.ExpirableVersion, expectFilterVersion, expectError bool) {
				shouldFilter, err := predicate(expirableVersion, version)
				if expectError {
					Expect(err).To(HaveOccurred())
					return
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(shouldFilter).To(Equal(expectFilterVersion))
			},

			// #FilterDifferentMajorMinorVersionAndLowerPatchVersionsOfSameMinor
			Entry("Should filter version - has not the same major.minor.",
				FilterDifferentMajorMinorVersionAndLowerPatchVersionsOfSameMinor(*semver.MustParse("1.2.0")),
				semver.MustParse("1.1.1"),
				gardencorev1beta1.ExpirableVersion{},
				true,
				false,
			),
			Entry("Should filter version - version has same major.minor but is lower",
				FilterDifferentMajorMinorVersionAndLowerPatchVersionsOfSameMinor(*semver.MustParse("1.1.2")),
				semver.MustParse("1.1.1"),
				gardencorev1beta1.ExpirableVersion{},
				true,
				false,
			),
			Entry("Should not filter version - has the same major.minor.",
				FilterDifferentMajorMinorVersionAndLowerPatchVersionsOfSameMinor(*semver.MustParse("1.1.0")),
				semver.MustParse("1.1.1"),
				gardencorev1beta1.ExpirableVersion{},
				false,
				false,
			),

			// #FilterNonConsecutiveMinorVersion
			Entry("Should filter version - has not the consecutive minor version.",
				FilterNonConsecutiveMinorVersion(*semver.MustParse("1.3.0")),
				semver.MustParse("1.1.1"),
				gardencorev1beta1.ExpirableVersion{},
				true,
				false,
			),
			Entry("Should filter version - has the same minor version.",
				FilterNonConsecutiveMinorVersion(*semver.MustParse("1.1.0")),
				semver.MustParse("1.1.1"),
				gardencorev1beta1.ExpirableVersion{},
				true,
				false,
			),
			Entry("Should not filter version - has consecutive minor.",
				FilterNonConsecutiveMinorVersion(*semver.MustParse("1.1.0")),
				semver.MustParse("1.2.0"),
				gardencorev1beta1.ExpirableVersion{},
				false,
				false,
			),

			// #FilterSameVersion
			Entry("Should filter version.",
				FilterSameVersion(*semver.MustParse("1.1.1")),
				semver.MustParse("1.1.1"),
				gardencorev1beta1.ExpirableVersion{},
				true,
				false,
			),
			Entry("Should not filter version.",
				FilterSameVersion(*semver.MustParse("1.1.1")),
				semver.MustParse("1.1.2"),
				gardencorev1beta1.ExpirableVersion{},
				false,
				false,
			),

			// #FilterExpiredVersion
			Entry("Should filter expired version.",
				FilterExpiredVersion(),
				nil,
				gardencorev1beta1.ExpirableVersion{
					ExpirationDate: &expirationDateInThePast,
				},
				true,
				false,
			),
			Entry("Should not filter version - expiration date is not expired",
				FilterExpiredVersion(),
				nil,
				gardencorev1beta1.ExpirableVersion{
					ExpirationDate: &metav1.Time{Time: time.Now().Add(time.Hour)},
				},
				false,
				false,
			),
			Entry("Should not filter version.",
				FilterExpiredVersion(),
				nil,
				gardencorev1beta1.ExpirableVersion{},
				false,
				false,
			),
			// #FilterDeprecatedVersion
			Entry("Should filter version - version is deprecated",
				FilterDeprecatedVersion(),
				nil,
				gardencorev1beta1.ExpirableVersion{Classification: &deprecatedClassification},
				true,
				false,
			),
			Entry("Should not filter version - version has preview classification",
				FilterDeprecatedVersion(),
				nil,
				gardencorev1beta1.ExpirableVersion{Classification: &previewClassification},
				false,
				false,
			),
			Entry("Should not filter version - version has supported classification",
				FilterDeprecatedVersion(),
				nil,
				gardencorev1beta1.ExpirableVersion{Classification: &supportedClassification},
				false,
				false,
			),
			Entry("Should not filter version - version has no classification",
				FilterDeprecatedVersion(),
				nil,
				gardencorev1beta1.ExpirableVersion{},
				false,
				false,
			),
			// #FilterLowerVersion
			Entry("Should filter version - version is lower",
				FilterLowerVersion(*semver.MustParse("1.1.1")),
				semver.MustParse("1.1.0"),
				gardencorev1beta1.ExpirableVersion{},
				true,
				false,
			),
			Entry("Should not filter version - version is higher / equal",
				FilterLowerVersion(*semver.MustParse("1.1.1")),
				semver.MustParse("1.1.2"),
				gardencorev1beta1.ExpirableVersion{},
				false,
				false,
			),
			// #FilterEqualAndSmallerMinorVersion
			Entry("Should filter version - version has the same minor version",
				FilterEqualAndSmallerMinorVersion(*semver.MustParse("1.1.5")),
				semver.MustParse("1.1.6"),
				gardencorev1beta1.ExpirableVersion{},
				true,
				false,
			),
			Entry("Should filter version - version has smaller minor version",
				FilterEqualAndSmallerMinorVersion(*semver.MustParse("1.1.5")),
				semver.MustParse("1.0.0"),
				gardencorev1beta1.ExpirableVersion{},
				true,
				false,
			),
			Entry("Should not filter version - version has higher minor version",
				FilterEqualAndSmallerMinorVersion(*semver.MustParse("1.1.5")),
				semver.MustParse("1.2.0"),
				gardencorev1beta1.ExpirableVersion{},
				false,
				false,
			),
			// #FilterEqualAndSmallerMajorVersion
			Entry("Should filter version - version has the same major version",
				FilterEqualAndSmallerMajorVersion(*semver.MustParse("2.1.5")),
				semver.MustParse("2.3.6"),
				gardencorev1beta1.ExpirableVersion{},
				true,
				false,
			),
			Entry("Should filter version - version has smaller major version",
				FilterEqualAndSmallerMajorVersion(*semver.MustParse("2.1.5")),
				semver.MustParse("1.0.0"),
				gardencorev1beta1.ExpirableVersion{},
				true,
				false,
			),
			Entry("Should not filter version - version has higher major version",
				FilterEqualAndSmallerMajorVersion(*semver.MustParse("1.1.5")),
				semver.MustParse("2.2.0"),
				gardencorev1beta1.ExpirableVersion{},
				false,
				false,
			),
			// #FilterDifferentMajorVersion
			Entry("Should filter version - version has the higher major version",
				FilterDifferentMajorVersion(*semver.MustParse("1.1.5")),
				semver.MustParse("2.3.6"),
				gardencorev1beta1.ExpirableVersion{},
				true,
				false,
			),
			Entry("Should filter version - version has smaller major version",
				FilterDifferentMajorVersion(*semver.MustParse("2.1.5")),
				semver.MustParse("1.0.0"),
				gardencorev1beta1.ExpirableVersion{},
				true,
				false,
			),
			Entry("Should not filter version - version has the same major version",
				FilterDifferentMajorVersion(*semver.MustParse("2.1.5")),
				semver.MustParse("2.2.0"),
				gardencorev1beta1.ExpirableVersion{},
				false,
				false,
			),
		)

		DescribeTable("#GetArchitecturesFromImageVersion",
			func(valuesInCapabilitySets, valuesInArchitecturesField, expectedResult []string) {
				imageVersion := gardencorev1beta1.MachineImageVersion{
					Architectures: valuesInArchitecturesField,
				}

				for _, architecture := range valuesInCapabilitySets {
					imageVersion.CapabilitySets = append(imageVersion.CapabilitySets, gardencorev1beta1.CapabilitySet{
						Capabilities: gardencorev1beta1.Capabilities{"architecture": gardencorev1beta1.CapabilityValues{architecture}},
					})
				}

				Expect(GetArchitecturesFromImageVersion(imageVersion)).To(ConsistOf(expectedResult))
			},
			Entry("Should return nil", nil, nil, nil),
			Entry("Should return architecture in set", []string{"amd64", "arm64"}, []string{"ia-64"}, []string{"amd64", "arm64"}),
			Entry("Should fall back to architectures field", nil, []string{"amd64", "arm64"}, []string{"amd64", "arm64"}),
		)

		DescribeTable("#ArchitectureSupportedByImageVersion",
			func(supportedArchitectures []string, requestedArchitecture string, expectedResult bool) {
				imageVersion := gardencorev1beta1.MachineImageVersion{
					Architectures: supportedArchitectures,
				}

				for _, architecture := range supportedArchitectures {
					imageVersion.CapabilitySets = append(imageVersion.CapabilitySets, gardencorev1beta1.CapabilitySet{
						Capabilities: gardencorev1beta1.Capabilities{"architecture": gardencorev1beta1.CapabilityValues{architecture}},
					})
				}

				Expect(ArchitectureSupportedByImageVersion(imageVersion, requestedArchitecture)).To(Equal(expectedResult))
			},
			Entry("Should be false for void architectures", nil, "arm64", false),
			Entry("Should be false for unsupported architecture", []string{"amd64", "arm64"}, "ia-64", false),
			Entry("Should be true for supported architecture", []string{"amd64", "arm64"}, "arm64", true),
		)
	})
})
