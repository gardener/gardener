// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

// MachineSpec define all bastion vm details derived from the CloudProfile
type MachineSpec struct {
	MachineTypeName string
	Architecture    string
	ImageBaseName   string
	ImageVersion    string
}

// GetMachineSpecFromCloudProfile determines the bastion vm details based on information in the cloud profile
func GetMachineSpecFromCloudProfile(profile *gardencorev1beta1.CloudProfile) (vm MachineSpec, err error) {
	if profile == nil {
		return MachineSpec{}, fmt.Errorf("cloudprofile is nil")
	}
	bastionSpec := profile.Spec.Bastion

	if bastionSpec != nil && bastionSpec.MachineType != nil {
		vm.MachineTypeName, vm.Architecture, err = getMachine(bastionSpec, profile.Spec.MachineTypes)
		if err != nil {
			return MachineSpec{}, err
		}
	} else {
		vm.MachineTypeName, vm.Architecture, err = findMostSuitableMachineType(profile)
		if err != nil {
			return MachineSpec{}, err
		}
	}

	vm.ImageBaseName, err = getImageName(bastionSpec, profile.Spec.MachineImages, vm.Architecture)
	if err != nil {
		return MachineSpec{}, err
	}
	vm.ImageVersion, err = getImageVersion(bastionSpec, vm.ImageBaseName, vm.Architecture, profile.Spec.MachineImages)
	return vm, err
}

// getMachine retrieves the bastion machine name and arch
func getMachine(bastion *gardencorev1beta1.Bastion, machineTypes []gardencorev1beta1.MachineType) (machineName string, machineArch string, err error) {
	machineIndex := slices.IndexFunc(machineTypes, func(machine gardencorev1beta1.MachineType) bool {
		return machine.Name == bastion.MachineType.Name
	})

	if machineIndex == -1 {
		return "", "",
			fmt.Errorf("bastion machine with name %s not found in cloudProfile", bastion.MachineType.Name)
	}

	machine := machineTypes[machineIndex]
	machineArch = machine.GetArchitecture()
	if machineArch == "" {
		return "", "",
			fmt.Errorf("architecture for specified bastion machine type %s is <nil>", bastion.MachineType.Name)
	}
	return machine.Name, machineArch, nil
}

func findSupportedArchitectures(images []gardencorev1beta1.MachineImage, machineImageName, machineImageVersion string) []string {
	architectures := sets.New[string]()

	for _, image := range images {
		if machineImageName != "" && image.Name != machineImageName {
			// Skip images that are not the specified one.
			continue
		}
		for _, version := range image.Versions {
			if machineImageVersion != "" && version.Version != machineImageVersion {
				// Skip versions that are not the specified one.
				continue
			}
			if version.Classification != nil && *version.Classification == gardencorev1beta1.ClassificationSupported {
				architectures.Insert(v1beta1helper.GetArchitecturesFromImageVersion(version)...)
			}
			if machineImageVersion != "" {
				// If an image version has been specified, we have now found it and extracted the architectures.
				break
			}
		}
		if machineImageName != "" {
			// If an image name has been specified, we have now found it and extracted the architectures.
			break
		}
	}

	return maps.Keys(architectures)
}

// getImageArchitectures finds the supported architectures of the cloudProfile images
// returning an empty array means all architectures are allowed
func getImageArchitectures(bastion *gardencorev1beta1.Bastion, images []gardencorev1beta1.MachineImage) []string {
	var (
		machineImageName, machineImageVersion string
	)
	// If bastion or bastion.Image is nil: find all supported architectures of all images.
	// Else, find all supported architectures of the specified image.
	if bastion != nil && bastion.MachineImage != nil {
		machineImageName = bastion.MachineImage.Name
		machineImageVersion = ptr.Deref(bastion.MachineImage.Version, "")
	}

	return findSupportedArchitectures(images, machineImageName, machineImageVersion)
}

// getImageName returns the image name for the bastion.
func getImageName(bastion *gardencorev1beta1.Bastion, images []gardencorev1beta1.MachineImage, arch string) (string, error) {
	// check if image name exists is also done in gardener cloudProfile validation
	if bastion != nil && bastion.MachineImage != nil {
		image, err := findImageByName(images, bastion.MachineImage.Name)
		if err != nil {
			return "", err
		}
		return image.Name, nil
	}

	// take the first image from cloud profile that is supported and arch compatible
	for _, image := range images {
		for _, version := range image.Versions {
			if version.Classification == nil || *version.Classification != gardencorev1beta1.ClassificationSupported {
				continue
			}
			if !slices.Contains(v1beta1helper.GetArchitecturesFromImageVersion(version), arch) {
				continue
			}
			return image.Name, nil
		}
	}
	return "", fmt.Errorf("could not find any supported bastion image for arch %s", arch)
}

// getImageVersion returns the image version for the bastion.
func getImageVersion(bastion *gardencorev1beta1.Bastion, imageName, machineArch string, images []gardencorev1beta1.MachineImage) (string, error) {
	image, err := findImageByName(images, imageName)
	if err != nil {
		return "", err
	}

	// check if image version exists is also done in gardener cloudProfile validation
	if bastion != nil && bastion.MachineImage != nil && bastion.MachineImage.Version != nil {
		versionIndex := slices.IndexFunc(image.Versions, func(version gardencorev1beta1.MachineImageVersion) bool {
			return version.Version == *bastion.MachineImage.Version
		})

		if versionIndex == -1 {
			return "", fmt.Errorf("image version %s not found not found in cloudProfile", *bastion.MachineImage.Version)
		}

		if image.Versions[versionIndex].Classification != nil && *image.Versions[versionIndex].Classification != gardencorev1beta1.ClassificationSupported {
			return "", fmt.Errorf("specified image %s in version %s is not classified supported", imageName, *bastion.MachineImage.Version)
		}

		return *bastion.MachineImage.Version, nil
	}

	var greatest *semver.Version
	for _, version := range image.Versions {
		if version.Classification == nil || *version.Classification != gardencorev1beta1.ClassificationSupported {
			continue
		}

		if !slices.Contains(v1beta1helper.GetArchitecturesFromImageVersion(version), machineArch) {
			continue
		}

		v, err := semver.NewVersion(version.Version)
		if err != nil {
			return "", err
		}

		if greatest == nil || v.GreaterThan(greatest) {
			greatest = v
		}
	}

	if greatest == nil {
		return "", fmt.Errorf("could not find any supported image version for %s and arch %s", imageName, machineArch)
	}
	return greatest.String(), nil
}

// findMostSuitableMachineType searches for the machine type that satisfies certain criteria
// currently we try to find the machine with the lowest amount of cpus
func findMostSuitableMachineType(profile *gardencorev1beta1.CloudProfile) (machineName string, machineArch string, err error) {
	supportedArchs := getImageArchitectures(profile.Spec.Bastion, profile.Spec.MachineImages)

	var minCpu *int64

	for _, machine := range profile.Spec.MachineTypes {
		arch := machine.GetArchitecture()
		if arch == "" {
			continue
		}

		if minCpu == nil || machine.CPU.Value() < *minCpu &&
			(supportedArchs == nil || slices.Contains(supportedArchs, arch)) {
			minCpu = ptr.To(machine.CPU.Value())
			machineName = machine.Name
			machineArch = arch
		}
	}

	if minCpu == nil {
		return "", "", fmt.Errorf("no suitable machine found")
	}

	return
}

// findImageByName returns image object found by name in the cloudProfile
func findImageByName(images []gardencorev1beta1.MachineImage, name string) (gardencorev1beta1.MachineImage, error) {
	imageIndex := slices.IndexFunc(images, func(image gardencorev1beta1.MachineImage) bool {
		return image.Name == name
	})

	if imageIndex == -1 {
		return gardencorev1beta1.MachineImage{}, fmt.Errorf("bastion image %s not found in cloudProfile", name)
	}

	return images[imageIndex], nil
}
