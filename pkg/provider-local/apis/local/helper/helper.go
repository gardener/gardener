// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/provider-local/apis/local"
)

// FindImageFromCloudProfile takes a cloud profile config and image details to find the appropriate
// image based on compatibility with the requested capabilities. If capabilities are specified,
// it performs capability matching to find the most suitable image.
func FindImageFromCloudProfile(
	cloudProfileConfig *local.CloudProfileConfig,
	name, version string,
	machineCapabilities v1beta1.Capabilities,
	capabilitiesDefinitions []v1beta1.CapabilityDefinition,
) (*local.MachineImageFlavor, error) {
	if cloudProfileConfig == nil {
		return nil, fmt.Errorf("cloud profile config is nil")
	}
	machineImages := cloudProfileConfig.MachineImages

	imageFlavor, err := findMachineImageFlavor(machineImages, name, version, machineCapabilities, capabilitiesDefinitions)
	if err != nil {
		return nil, fmt.Errorf("could not find image %q, version %q that supports %v: %w", name, version, machineCapabilities, err)
	}

	if imageFlavor != nil {
		return imageFlavor, nil
	}
	return nil, fmt.Errorf("could not find image %q, version %q that supports %v", name, version, machineCapabilities)
}

func findMachineImageFlavor(
	machineImages []local.MachineImages,
	imageName, imageVersion string,
	machineCapabilities v1beta1.Capabilities,
	capabilitiesDefinitions []v1beta1.CapabilityDefinition,
) (*local.MachineImageFlavor, error) {
	for _, machineImage := range machineImages {
		if machineImage.Name != imageName {
			continue
		}

		for _, version := range machineImage.Versions {
			if imageVersion != version.Version {
				continue
			}

			// If no capabilitiesDefinitions are specified, return the (legacy) image field as no flavors are used.
			if len(capabilitiesDefinitions) == 0 {
				return &local.MachineImageFlavor{
					Image:        version.Image,
					Capabilities: v1beta1.Capabilities{},
				}, nil
			}

			bestMatch, err := worker.FindBestImageFlavor(version.Flavors, machineCapabilities, capabilitiesDefinitions)
			if err != nil {
				return nil, fmt.Errorf("could not determine best flavor %w", err)
			}

			return &bestMatch, nil
		}
	}
	return nil, nil
}
