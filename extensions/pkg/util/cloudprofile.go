// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"maps"
	"slices"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
)

// ImagesContext is a helper struct to consume cloud profile images and their versions.
type ImagesContext[A any, B any] struct {
	Images map[string]A

	createVersionsMap func(A) map[string]B
	// imageVersions will be calculated lazily on first access of each key.
	imageVersions map[string]map[string]B
}

// GetImage returns the image with the given name.
func (vc *ImagesContext[A, B]) GetImage(imageName string) (A, bool) {
	o, exists := vc.Images[imageName]
	return o, exists
}

// GetImageVersion returns the image version with the given name and version.
func (vc *ImagesContext[A, B]) GetImageVersion(imageName string, version string) (B, bool) {
	o, exists := vc.getImageVersions(imageName)[version]
	return o, exists
}

func (vc *ImagesContext[A, B]) getImageVersions(imageName string) map[string]B {
	if versions, exists := vc.imageVersions[imageName]; exists {
		return versions
	}
	vc.imageVersions[imageName] = vc.createVersionsMap(vc.Images[imageName])
	return vc.imageVersions[imageName]
}

// NewImagesContext creates a new generic ImagesContext.
func NewImagesContext[A any, B any](images map[string]A, createVersionsMap func(A) map[string]B) *ImagesContext[A, B] {
	return &ImagesContext[A, B]{
		Images:            images,
		createVersionsMap: createVersionsMap,
		imageVersions:     make(map[string]map[string]B),
	}
}

// NewCoreImagesContext creates a new ImagesContext for core.MachineImage.
func NewCoreImagesContext(profileImages []core.MachineImage) *ImagesContext[core.MachineImage, core.MachineImageVersion] {
	return NewImagesContext(
		utils.CreateMapFromSlice(profileImages, func(mi core.MachineImage) string { return mi.Name }),
		func(mi core.MachineImage) map[string]core.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v core.MachineImageVersion) string { return v.Version })
		},
	)
}

// NewV1beta1ImagesContext creates a new ImagesContext for gardencorev1beta1.MachineImage.
func NewV1beta1ImagesContext(parentImages []gardencorev1beta1.MachineImage) *ImagesContext[gardencorev1beta1.MachineImage, gardencorev1beta1.MachineImageVersion] {
	return NewImagesContext(
		utils.CreateMapFromSlice(parentImages, func(mi gardencorev1beta1.MachineImage) string { return mi.Name }),
		func(mi gardencorev1beta1.MachineImage) map[string]gardencorev1beta1.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v gardencorev1beta1.MachineImageVersion) string { return v.Version })
		},
	)
}

// ValidateCapabilities validates the capabilities of a machine type or machine image against the capabilitiesDefinition
// located in a cloud profile at spec.capabilities.
// It checks if the capabilities are supported by the cloud profile and if the architecture is defined correctly.
// It returns a list of field errors if any validation fails.
func ValidateCapabilities(capabilities core.Capabilities, capabilitiesDefinition core.Capabilities, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	supportedCapabilityKeys := slices.Collect(maps.Keys(capabilitiesDefinition))

	// Check if all capabilities are supported by the cloud profile
	for capabilityKey, capability := range capabilities {
		supportedValues, keyExists := capabilitiesDefinition[capabilityKey]
		if !keyExists {
			allErrs = append(allErrs, field.NotSupported(fldPath, capabilityKey, supportedCapabilityKeys))
			continue
		}
		for i, value := range capability {
			if !slices.Contains(supportedValues, value) {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child(capabilityKey).Index(i), value, supportedValues))
			}
		}
	}

	// Check additional requirements for architecture
	// must be defined when multiple architectures are supported by the cloud profile
	supportedArchitectures := capabilitiesDefinition[v1beta1constants.ArchitectureName]
	architectures := capabilities[v1beta1constants.ArchitectureName]
	if len(supportedArchitectures) > 1 && len(architectures) != 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child(v1beta1constants.ArchitectureName), architectures, "must define exactly one architecture when multiple architectures are supported by the cloud profile"))
	}

	return allErrs
}

// GetVersionCapabilitySets returns the CapabilitySets for a given machine image version or adds the default capabilitySet if non is defined.
func GetVersionCapabilitySets(version core.MachineImageVersion, capabilitiesDefinition core.Capabilities) []core.CapabilitySet {
	versionCapabilitySets := version.CapabilitySets
	if len(version.CapabilitySets) == 0 {
		// It is allowed not to define capabilitySets in the machine image version if there is only one architecture
		// if so the capabilityDefinition is used as default
		if len(capabilitiesDefinition[v1beta1constants.ArchitectureName]) == 1 {
			versionCapabilitySets = []core.CapabilitySet{{Capabilities: capabilitiesDefinition}}
		}
	}
	return versionCapabilitySets
}

// AreCapabilitiesEqual checks if two capabilities are semantically equal.
// It compares the keys and values of the capabilities maps.
func AreCapabilitiesEqual(a, b, capabilitiesDefinition core.Capabilities) bool {
	a = SetDefaultCapabilities(a, capabilitiesDefinition)
	b = SetDefaultCapabilities(b, capabilitiesDefinition)
	for key, valuesA := range a {
		valuesB, exists := b[key]
		if !exists || len(valuesA) != len(valuesB) {
			return false
		}
		for _, value := range valuesA {
			if !slices.Contains(valuesB, value) {
				return false
			}
		}
	}
	return true
}

// SetDefaultCapabilities sets the default capabilities based on a capabilitiesDefinition for a machine type or machine image.
func SetDefaultCapabilities(capabilities, capabilitiesDefinition core.Capabilities) core.Capabilities {
	if len(capabilities) == 0 {
		capabilities = make(core.Capabilities)
	}

	for key, values := range capabilitiesDefinition {
		if _, exists := capabilities[key]; !exists {
			capabilities[key] = values
		}
	}

	return capabilities
}
