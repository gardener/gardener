// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion_test

import (
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/extensions/pkg/bastion"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Bastion VM Details", func() {
	var cloudProfile *gardencorev1beta1.CloudProfile
	var desired MachineSpec

	BeforeEach(func() {
		desired = MachineSpec{
			MachineTypeName: "small_machine",
			Architecture:    "amd64",
			ImageBaseName:   "gardenlinux",
			ImageVersion:    "1.2.3",
		}
		cloudProfile = &gardencorev1beta1.CloudProfile{
			Spec: gardencorev1beta1.CloudProfileSpec{
				Bastion: &gardencorev1beta1.Bastion{
					MachineImage: &gardencorev1beta1.BastionMachineImage{
						Name: desired.ImageBaseName,
					},
					MachineType: &gardencorev1beta1.BastionMachineType{
						Name: desired.MachineTypeName,
					},
				},
				MachineTypes: []gardencorev1beta1.MachineType{{
					CPU:          resource.MustParse("4"),
					Name:         desired.MachineTypeName,
					Architecture: ptr.To(desired.Architecture),
				}},
				MachineImages: []gardencorev1beta1.MachineImage{{
					Name: desired.ImageBaseName,
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        desired.ImageVersion,
								Classification: ptr.To(gardencorev1beta1.ClassificationSupported),
							},
							Architectures: []string{desired.Architecture, "arm64"},
						}},
				}},
			},
		}
	})

	addImageToCloudProfile := func(imageName, version string, classification gardencorev1beta1.VersionClassification, archs []string) {
		machineIndex := slices.IndexFunc(cloudProfile.Spec.MachineImages, func(image gardencorev1beta1.MachineImage) bool {
			return image.Name == imageName
		})

		newVersion := gardencorev1beta1.MachineImageVersion{
			ExpirableVersion: gardencorev1beta1.ExpirableVersion{
				Version:        version,
				Classification: ptr.To(classification),
			},
			Architectures: archs,
		}

		// append new machine image
		if machineIndex == -1 {
			cloudProfile.Spec.MachineImages = append(cloudProfile.Spec.MachineImages, gardencorev1beta1.MachineImage{
				Name:     imageName,
				Versions: []gardencorev1beta1.MachineImageVersion{newVersion},
			})
		}

		// add new version
		cloudProfile.Spec.MachineImages[machineIndex].Versions = append(cloudProfile.Spec.MachineImages[machineIndex].Versions, newVersion)
	}

	Describe("#GetMachineSpecFromCloudProfile", func() {
		It("should succeed without setting bastion image version", func() {
			details, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("should succeed with empty bastion section", func() {
			cloudProfile.Spec.Bastion = &gardencorev1beta1.Bastion{}
			details, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("should succeed without setting bastion section", func() {
			cloudProfile.Spec.Bastion = nil
			details, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("should succeed without setting bastion image", func() {
			cloudProfile.Spec.Bastion.MachineImage = nil
			details, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("should succeed without setting machineType", func() {
			cloudProfile.Spec.Bastion.MachineType = nil
			details, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("forbid unknown image name", func() {
			cloudProfile.Spec.Bastion.MachineImage.Name = "unknown_image"
			_, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).To(HaveOccurred())
		})

		It("forbid unknown image version", func() {
			cloudProfile.Spec.Bastion.MachineImage.Version = ptr.To("6.6.6")
			_, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).To(HaveOccurred())
		})

		It("forbid unknown machineType", func() {
			cloudProfile.Spec.Bastion.MachineType.Name = "unknown_machine"
			_, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).To(HaveOccurred())
		})

		It("should find greatest supported version", func() {
			addImageToCloudProfile(desired.ImageBaseName, "1.2.4", gardencorev1beta1.ClassificationSupported, []string{"amd64"})
			desired.ImageVersion = "1.2.4"
			details, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("should find smallest machine", func() {
			cloudProfile.Spec.Bastion.MachineType = nil
			cloudProfile.Spec.MachineTypes = append(cloudProfile.Spec.MachineTypes, gardencorev1beta1.MachineType{
				CPU:          resource.MustParse("1"),
				GPU:          resource.MustParse("1"),
				Name:         "smallerMachine",
				Architecture: ptr.To(desired.Architecture),
			})
			details, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(details.MachineTypeName).To(DeepEqual("smallerMachine"))
		})

		It("should only use supported version", func() {
			addImageToCloudProfile(desired.ImageBaseName, "1.2.4", gardencorev1beta1.ClassificationPreview, []string{"amd64"})
			details, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("should use version which has been specified", func() {
			addImageToCloudProfile(desired.ImageBaseName, "1.2.4", gardencorev1beta1.ClassificationSupported, []string{"amd64"})
			cloudProfile.Spec.Bastion.MachineImage.Version = ptr.To("1.2.3")
			details, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("should not allow preview image even if version is specified", func() {
			addImageToCloudProfile(desired.ImageBaseName, "1.2.4", gardencorev1beta1.ClassificationPreview, []string{"amd64"})
			cloudProfile.Spec.Bastion.MachineImage.Version = ptr.To("1.2.4")
			_, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).To(HaveOccurred())
		})

		It("only use images for matching machineType architecture", func() {
			addImageToCloudProfile(desired.ImageBaseName, "1.2.4", gardencorev1beta1.ClassificationSupported, []string{"x86"})
			details, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).NotTo(HaveOccurred())
			Expect(details).To(DeepEqual(desired))
		})

		It("fail if no image with matching machineType architecture can be found", func() {
			cloudProfile.Spec.MachineImages[0].Versions[0].Architectures = []string{"x86"}
			_, err := GetMachineSpecFromCloudProfile(cloudProfile)
			Expect(err).To(HaveOccurred())
		})
	})
})
