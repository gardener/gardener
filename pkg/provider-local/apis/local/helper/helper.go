// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"
	"slices"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/provider-local/apis/local"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1"
)

// FindImageFromCloudProfile takes a cloud profile config and image details to find the appropriate
// image based on compatibility with the requested capabilities. If capabilities are specified,
// it performs capability matching to find the most suitable image.
func FindImageFromCloudProfile(
	cloudProfileConfig *local.CloudProfileConfig,
	name, version string,
	machineCapabilities v1beta1.Capabilities,
	capabilitiesDefinitions []v1beta1.CapabilityDefinition,
) (*local.CapabilitySet, error) {
	if cloudProfileConfig == nil {
		return nil, fmt.Errorf("cloud profile config is nil")
	}
	machineImages := cloudProfileConfig.MachineImages

	capabilitySet, err := findCapabilitySetFromMachineImages(machineImages, name, version, machineCapabilities, capabilitiesDefinitions)
	if err != nil {
		return nil, fmt.Errorf("could not find image %q, version %q that supports %v: %w", name, version, machineCapabilities, err)
	}

	if capabilitySet != nil {
		return capabilitySet, nil
	}
	return nil, fmt.Errorf("could not find image %q, version %q that supports %v", name, version, machineCapabilities)
}

func findCapabilitySetFromMachineImages(
	machineImages []local.MachineImages,
	imageName, imageVersion string,
	machineCapabilities v1beta1.Capabilities,
	capabilitiesDefinitions []v1beta1.CapabilityDefinition,
) (*local.CapabilitySet, error) {
	for _, machineImage := range machineImages {
		if machineImage.Name != imageName {
			continue
		}

		for _, version := range machineImage.Versions {
			if imageVersion != version.Version {
				continue
			}

			// If no capabilitiesDefinitions are specified, return the (legacy) image field as no capabilitySets are used.
			if len(capabilitiesDefinitions) == 0 {
				return &local.CapabilitySet{
					Image:        version.Image,
					Capabilities: core.Capabilities{},
				}, nil
			}

			bestMatch, err := FindBestCapabilitySet(version.CapabilitySets, machineCapabilities, capabilitiesDefinitions)
			if err != nil {
				return nil, fmt.Errorf("could not determine best capabilitySet %w", err)
			}

			return bestMatch, nil
		}
	}
	return nil, nil
}

// FindBestCapabilitySet finds the most appropriate capability set from the provided capability sets
// based on the requested machine capabilities and the definitions of capabilities.
func FindBestCapabilitySet(
	capabilitySets []local.CapabilitySet,
	machineCapabilities v1beta1.Capabilities,
	capabilitiesDefinitions []v1beta1.CapabilityDefinition,
) (*local.CapabilitySet, error) {
	compatibleCapabilitySets, err := findCompatibleCapabilitySets(capabilitySets, machineCapabilities, capabilitiesDefinitions)
	if err != nil {
		return nil, err
	}

	if len(compatibleCapabilitySets) == 0 {
		return nil, fmt.Errorf("no compatible capability set found")
	}

	bestMatch, err := selectBestCapabilitySet(compatibleCapabilitySets, capabilitiesDefinitions)
	if err != nil {
		return nil, err
	}
	return bestMatch, nil
}

// findCompatibleCapabilitySets returns all capability sets that are compatible with the given machine capabilities.
func findCompatibleCapabilitySets(
	capabilitySets []local.CapabilitySet,
	machineCapabilities v1beta1.Capabilities,
	capabilitiesDefinitions []v1beta1.CapabilityDefinition,
) ([]local.CapabilitySet, error) {
	var compatibleSets []local.CapabilitySet

	for _, capabilitySet := range capabilitySets {
		var v1alphaCapabilitySet v1alpha1.CapabilitySet
		err := v1alpha1.Convert_local_CapabilitySet_To_v1alpha1_CapabilitySet(&capabilitySet, &v1alphaCapabilitySet, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to convert capability set: %w", err)
		}

		if v1beta1helper.AreCapabilitiesCompatible(v1alphaCapabilitySet.Capabilities, machineCapabilities, capabilitiesDefinitions) {
			compatibleSets = append(compatibleSets, capabilitySet)
		}
	}

	return compatibleSets, nil
}

// selectBestCapabilitySet selects the most appropriate capability set based on the priority
// of capabilities and their values as defined in capabilitiesDefinitions.
//
// Selection follows a priority-based approach:
// 1. Capabilities are ordered by priority in the definitions list (highest priority first)
// 2. Within each capability, values are ordered by preference (most preferred first)
// 3. Selection is determined by the first capability value difference found
func selectBestCapabilitySet(
	compatibleSets []local.CapabilitySet,
	capabilitiesDefinitions []v1beta1.CapabilityDefinition,
) (*local.CapabilitySet, error) {
	if len(compatibleSets) == 1 {
		return &compatibleSets[0], nil
	}

	// Apply capability defaults for better comparison
	normalizedSets := make([]local.CapabilitySet, len(compatibleSets))
	copy(normalizedSets, compatibleSets)

	coreCapabilitiesDefinitions, err := gardencorehelper.ConvertCoreCapabilitiesDefinitions(capabilitiesDefinitions)
	if err != nil {
		return nil, err
	}

	// Normalize capability sets by applying defaults
	for i := range normalizedSets {
		normalizedSets[i].Capabilities = gardencorehelper.GetCapabilitiesWithAppliedDefaults(
			normalizedSets[i].Capabilities,
			coreCapabilitiesDefinitions,
		)
	}

	// Evaluate capability sets based on capability definitions priority
	remainingSets := normalizedSets

	// For each capability (in priority order)
	for _, capabilityDef := range capabilitiesDefinitions {
		// For each preferred value (in preference order)
		for _, capabilityValue := range capabilityDef.Values {
			var setsWithPreferredValue []local.CapabilitySet

			// Find sets that support this capability value
			for _, set := range remainingSets {
				if slices.Contains(set.Capabilities[capabilityDef.Name], capabilityValue) {
					setsWithPreferredValue = append(setsWithPreferredValue, set)
				}
			}

			// If we found sets with this value, narrow down our selection
			if len(setsWithPreferredValue) > 0 {
				remainingSets = setsWithPreferredValue

				// If only one set remains, we've found our match
				if len(remainingSets) == 1 {
					return &remainingSets[0], nil
				}
			}
		}
	}

	// If we couldn't determine a single best match, this indicates a problem with the cloud profile
	if len(remainingSets) != 1 {
		return nil, fmt.Errorf("found multiple capability sets with identical capabilities; this indicates an invalid cloudprofile was admitted. Please open a bug report at https://github.com/gardener/gardener/issues")
	}

	return &remainingSets[0], nil
}
