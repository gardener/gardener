// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"fmt"
	"slices"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

// HasCapabilities defines an interface for types that contain Capabilities
type HasCapabilities interface {
	GetCapabilities() v1beta1.Capabilities
}

// FindBestCapabilitySet finds the most appropriate capability set from the provided capability sets
// based on the requested machine capabilities and the definitions of capabilities.
func FindBestCapabilitySet[T HasCapabilities](
	capabilitySets []T,
	machineCapabilities v1beta1.Capabilities,
	capabilitiesDefinitions []v1beta1.CapabilityDefinition,
) (T, error) {
	var zeroValue T
	compatibleCapabilitySets := filterCompatibleCapabilitySets(capabilitySets, machineCapabilities, capabilitiesDefinitions)

	if len(compatibleCapabilitySets) == 0 {
		return zeroValue, fmt.Errorf("no compatible capability set found")
	}

	bestMatch, err := selectBestCapabilitySet(compatibleCapabilitySets, capabilitiesDefinitions)
	if err != nil {
		return zeroValue, err
	}
	return bestMatch, nil
}

// filterCompatibleCapabilitySets returns all capability sets that are compatible with the given machine capabilities.
func filterCompatibleCapabilitySets[T HasCapabilities](
	capabilitySets []T, machineCapabilities v1beta1.Capabilities, capabilitiesDefinitions []v1beta1.CapabilityDefinition,
) []T {
	var compatibleSets []T
	for _, capabilitySet := range capabilitySets {
		if v1beta1helper.AreCapabilitiesCompatible(capabilitySet.GetCapabilities(), machineCapabilities, capabilitiesDefinitions) {
			compatibleSets = append(compatibleSets, capabilitySet)
		}
	}
	return compatibleSets
}

// selectBestCapabilitySet selects the most appropriate capability set based on the priority
// of capabilities and their values as defined in capabilitiesDefinitions.
//
// Selection follows a priority-based approach:
// 1. Capabilities are ordered by priority in the definitions list (highest priority first)
// 2. Within each capability, values are ordered by preference (most preferred first)
// 3. Selection is determined by the first capability value difference found
func selectBestCapabilitySet[T HasCapabilities](
	compatibleSets []T,
	capabilitiesDefinitions []v1beta1.CapabilityDefinition,
) (T, error) {
	var zeroValue T
	if len(compatibleSets) == 1 {
		return compatibleSets[0], nil
	}

	type capabilitiesWithProviderType struct {
		providerEntry T
		capabilities  v1beta1.Capabilities
	}

	var capabilitiesWithProviderTypes []capabilitiesWithProviderType
	for _, set := range compatibleSets {
		capabilitiesWithProviderTypes = append(capabilitiesWithProviderTypes, capabilitiesWithProviderType{
			providerEntry: set,
		})
	}
	// Normalize capabilities copy by applying defaults
	for i := range capabilitiesWithProviderTypes {
		capabilitiesWithProviderTypes[i].capabilities = v1beta1helper.GetCapabilitiesWithAppliedDefaults(capabilitiesWithProviderTypes[i].providerEntry.GetCapabilities(), capabilitiesDefinitions)
	}

	// Evaluate capability sets based on capability definitions priority
	remainingSets := capabilitiesWithProviderTypes

	// For each capability (in priority order)
	for _, capabilityDef := range capabilitiesDefinitions {
		// For each preferred value (in preference order)
		for _, capabilityValue := range capabilityDef.Values {
			var setsWithPreferredValue []capabilitiesWithProviderType

			// Find sets that support this capability value
			for _, set := range remainingSets {
				if slices.Contains(set.capabilities[capabilityDef.Name], capabilityValue) {
					setsWithPreferredValue = append(setsWithPreferredValue, set)
				}
			}

			// If we found sets with this value, narrow down our selection
			if len(setsWithPreferredValue) > 0 {
				remainingSets = setsWithPreferredValue

				// If only one set remains, we've found our match
				if len(remainingSets) == 1 {
					return remainingSets[0].providerEntry, nil
				}
			}
		}
	}

	// If we couldn't determine a single best match, this indicates a problem with the cloud profile
	if len(remainingSets) != 1 {
		return zeroValue, fmt.Errorf("found multiple capability sets with identical capabilities; this indicates an invalid cloudprofile was admitted. Please open a bug report at https://github.com/gardener/gardener/issues")
	}

	return remainingSets[0].providerEntry, nil
}
