// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
)

// FindMachineImage takes a list of machine images and tries to find the first entry
// whose name, version, and zone matches with the given name, version, and region. If no such entry is
// found then an error will be returned.
func FindMachineImage(machineImages []api.MachineImage, name, version string) (*api.MachineImage, error) {
	for _, machineImage := range machineImages {
		if machineImage.Name == name && machineImage.Version == version {
			return &machineImage, nil
		}
	}
	return nil, fmt.Errorf("no machine image with name %q, version %q found", name, version)
}

// FindImageFromCloudProfile takes a list of machine images, and the desired image name, version.. It tries to find the
// image with the given name and version in the desired region. If it cannot be found then an error is returned.
func FindImageFromCloudProfile(cloudProfileConfig *api.CloudProfileConfig, imageName, imageVersion string) (string, error) {
	if cloudProfileConfig != nil {
		for _, machineImage := range cloudProfileConfig.MachineImages {
			if machineImage.Name != imageName {
				continue
			}

			for _, version := range machineImage.Versions {
				if imageVersion != version.Version {
					continue
				}

				return version.Image, nil
			}
		}
	}

	return "", fmt.Errorf("could not find an image for name %q in version %q", imageName, imageVersion)
}
