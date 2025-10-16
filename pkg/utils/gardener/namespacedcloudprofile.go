// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// TransformSpecToParentFormat ensures that the given NamespacedCloudProfileSpec is in a uniform format with its parent CloudProfileSpec.
// If the parent CloudProfileSpec uses capability definitions, then the NamespacedCloudProfileSpec is transformed to also use capabilities
// and vice versa.
// TODO(Roncossek): Remove TransformSpecToParentFormat once all CloudProfiles have been migrated to use CapabilityFlavors and the Architecture fields are effectively forbidden or have been removed.
func TransformSpecToParentFormat(
	spec gardencorev1beta1.NamespacedCloudProfileSpec,
	capabilityDefinitions []gardencorev1beta1.CapabilityDefinition,
) gardencorev1beta1.NamespacedCloudProfileSpec {
	isParentInCapabilityFormat := len(capabilityDefinitions) > 0
	transformedSpec := spec.DeepCopy()

	// Normalize MachineImages
	for idx, machineImage := range transformedSpec.MachineImages {
		for idy, version := range machineImage.Versions {
			legacyArchitectures := version.Architectures

			if isParentInCapabilityFormat && len(version.CapabilityFlavors) == 0 {
				// Convert legacy architectures to capability flavors
				version.CapabilityFlavors = []gardencorev1beta1.MachineImageFlavor{}
				for _, arch := range legacyArchitectures {
					version.CapabilityFlavors = append(version.CapabilityFlavors, gardencorev1beta1.MachineImageFlavor{
						Capabilities: gardencorev1beta1.Capabilities{
							v1beta1constants.ArchitectureName: []string{arch},
						},
					})
				}
				version.Architectures = legacyArchitectures
			} else if !isParentInCapabilityFormat {
				// Convert capability flavors to legacy architectures
				if len(legacyArchitectures) == 0 {
					architectureSet := sets.New[string]()
					if len(version.CapabilityFlavors) > 0 {
						for _, flavor := range version.CapabilityFlavors {
							architectureSet.Insert(flavor.Capabilities[v1beta1constants.ArchitectureName]...)
						}
					}
					version.Architectures = architectureSet.UnsortedList()
				}
				version.CapabilityFlavors = nil
			}

			transformedSpec.MachineImages[idx].Versions[idy] = version
		}
	}

	// Normalize MachineTypes
	for idx, machineType := range transformedSpec.MachineTypes {
		if isParentInCapabilityFormat {
			if len(machineType.Capabilities) > 0 {
				continue
			}
			architecture := machineType.GetArchitecture(capabilityDefinitions)
			if architecture == "" {
				architecture = ptr.Deref(machineType.Architecture, v1beta1constants.ArchitectureAMD64)
			}
			machineType.Capabilities = gardencorev1beta1.Capabilities{
				v1beta1constants.ArchitectureName: []string{architecture},
			}
		} else {
			if machineType.Architecture == nil {
				if len(machineType.Capabilities) > 0 {
					architecture := machineType.Capabilities[v1beta1constants.ArchitectureName]
					if len(architecture) > 0 {
						machineType.Architecture = &architecture[0]
					}
				}
			}
			machineType.Capabilities = nil
		}
		transformedSpec.MachineTypes[idx] = machineType
	}
	return *transformedSpec
}
