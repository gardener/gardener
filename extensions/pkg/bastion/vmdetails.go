// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/exp/maps"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// VmDetails define all bastion vm details derived from the CloudProfile
type VMDetails struct {
	MachineName   string
	Architecture  string
	ImageBaseName string
	ImageVersion  string
}

// DetermineVmDetails determines the bastion vm details based on information in the cloud profile
func DetermineVmDetails(spec gardencorev1beta1.CloudProfileSpec) (vm VMDetails, err error) {
	imageArchs, err := getArchitectures(spec.Bastion, spec.MachineImages)
	if err != nil {
		return VMDetails{}, err
	}
	vm.MachineName, vm.Architecture, err = getMachine(spec.Bastion, spec.MachineTypes, imageArchs)
	if err != nil {
		return VMDetails{}, err
	}
	vm.ImageBaseName, err = getImageName(spec.Bastion, spec.MachineImages, vm.Architecture)
	if err != nil {
		return VMDetails{}, err
	}
	vm.ImageVersion, err = getImageVersion(vm.ImageBaseName, vm.Architecture, spec.Bastion, spec.MachineImages)
	return vm, err
}

// getMachine retrieves the bastion machine name and arch
func getMachine(bastion *gardencorev1beta1.Bastion, machineTypes []gardencorev1beta1.MachineType, supportedArchs []string) (machineName string, machineArch string, err error) {
	if bastion != nil && bastion.MachineType != nil {
		machineIndex := slices.IndexFunc(machineTypes, func(machine gardencorev1beta1.MachineType) bool {
			return machine.Name == bastion.MachineType.Name
		})

		if machineIndex == -1 {
			return "", "",
				fmt.Errorf("bastion machine with name %s not found in cloudProfile", bastion.MachineType.Name)
		}

		machine := machineTypes[machineIndex]
		return machine.Name, *machine.Architecture, nil
	}

	// find the machine in cloud profile with the lowest amount of cpus
	var minCpu *int64

	for _, machine := range machineTypes {
		if machine.Architecture == nil {
			continue
		}

		arch := *machine.Architecture
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

// getArchitectures finds the supported architectures of the cloudProfiles images
// returning an empty array means all architectures are allowed
func getArchitectures(bastion *gardencorev1beta1.Bastion, images []gardencorev1beta1.MachineImage) ([]string, error) {
	archs := make(map[string]bool)

	findSupportedArchs := func(versions []gardencorev1beta1.MachineImageVersion, bastionVersion *string) {
		for _, version := range versions {
			if bastionVersion != nil && version.Version == *bastionVersion {
				archs = make(map[string]bool)
				for _, arch := range version.Architectures {
					archs[arch] = true
				}
				return
			}

			if version.Classification != nil && *version.Classification == gardencorev1beta1.ClassificationSupported {
				for _, arch := range version.Architectures {
					archs[arch] = true
				}
			}
		}
	}

	// if no bastion or only bastion.MachineType is set: find all supported archs of all images
	if bastion == nil || bastion.MachineImage == nil {
		for _, image := range images {
			findSupportedArchs(image.Versions, nil)
		}
		return maps.Keys(archs), nil
	}

	// if only machineImage is set: find all supported versions if no version is set otherwise return arch of version
	if bastion.MachineImage != nil && bastion.MachineType == nil {
		image, err := findImageByName(images, bastion.MachineImage.Name)
		if err != nil {
			return nil, err
		}

		findSupportedArchs(image.Versions, bastion.MachineImage.Version)

		return maps.Keys(archs), nil
	}

	return nil, nil
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
			if !slices.Contains(version.Architectures, arch) {
				continue
			}
			return image.Name, nil
		}
	}
	return "", fmt.Errorf("could not find any supported bastion image for arch %s", arch)
}

// getImageVersion returns the image version for the bastion.
func getImageVersion(imageName, machineArch string, bastion *gardencorev1beta1.Bastion, images []gardencorev1beta1.MachineImage) (string, error) {
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

		if !slices.Contains(version.Architectures, machineArch) {
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
