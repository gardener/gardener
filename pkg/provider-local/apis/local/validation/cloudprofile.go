// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	coreapi "github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ValidateCloudProfileConfig validates a CloudProfileConfig object.
func ValidateCloudProfileConfig(cpConfig *api.CloudProfileConfig, machineImages []core.MachineImage, capabilitiesDefinitions []gardencorev1beta1.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	machineImagesPath := fldPath.Child("machineImages")

	// Validate machine images section
	allErrs = append(allErrs, validateMachineImages(cpConfig.MachineImages, capabilitiesDefinitions, machineImagesPath)...)
	if len(allErrs) > 0 {
		return allErrs
	}

	// Validate machine image mappings
	allErrs = append(allErrs, validateMachineImageMapping(machineImages, cpConfig.MachineImages, capabilitiesDefinitions, machineImagesPath)...)

	return allErrs
}

// validateMachineImages validates the machine images section of CloudProfileConfig
func validateMachineImages(machineImages []api.MachineImages, capabilitiesDefinitions []gardencorev1beta1.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure at least one machine image is provided
	if len(machineImages) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine image"))
		return allErrs
	}

	// Validate each machine image
	for i, machineImage := range machineImages {
		idxPath := fldPath.Index(i)
		allErrs = append(allErrs, validateMachineImage(machineImage, capabilitiesDefinitions, idxPath)...)
	}

	return allErrs
}

// validateMachineImage validates an individual machine image configuration
func validateMachineImage(machineImage api.MachineImages, capabilitiesDefinitions []gardencorev1beta1.CapabilityDefinition, idxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineImage.Name) == 0 {
		allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must provide a name"))
	}

	if len(machineImage.Versions) == 0 {
		allErrs = append(allErrs, field.Required(idxPath.Child("versions"),
			fmt.Sprintf("must provide at least one version for machine image %q", machineImage.Name)))
		return allErrs
	}

	// Validate each version
	for j, version := range machineImage.Versions {
		jdxPath := idxPath.Child("versions").Index(j)
		allErrs = append(allErrs, validateMachineImageVersion(version, capabilitiesDefinitions, jdxPath)...)
	}

	return allErrs
}

// validateMachineImageVersion validates a specific machine image version
func validateMachineImageVersion(version api.MachineImageVersion, capabilitiesDefinitions []gardencorev1beta1.CapabilityDefinition, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(version.Version) == 0 {
		allErrs = append(allErrs, field.Required(jdxPath.Child("version"), "must provide a version"))
	}

	// Different validation paths based on whether capabilities are defined
	if len(capabilitiesDefinitions) > 0 {
		allErrs = append(allErrs, validateWithCapabilities(version, capabilitiesDefinitions, jdxPath)...)
	} else {
		allErrs = append(allErrs, validateWithoutCapabilities(version, jdxPath)...)
	}

	return allErrs
}

// validateWithCapabilities validates a machine image version when capabilities are defined
func validateWithCapabilities(version api.MachineImageVersion, capabilitiesDefinitions []gardencorev1beta1.CapabilityDefinition, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// When using capabilities, image must not be set directly
	if len(version.Image) > 0 {
		allErrs = append(allErrs, field.Forbidden(jdxPath.Child("image"),
			"must not be set as CloudProfile defines capabilities. Use capabilitySets[].Image instead."))
	}

	// Validate each capability set
	for k, capabilitySet := range version.CapabilitySets {
		kdxPath := jdxPath.Child("capabilitySets").Index(k)
		if len(capabilitySet.Image) == 0 {
			allErrs = append(allErrs, field.Required(kdxPath.Child("image"), "must provide an image"))
		}
		allErrs = append(allErrs, v1beta1helper.ValidateCapabilities(capabilitySet.Capabilities, capabilitiesDefinitions, kdxPath.Child("capabilities"))...)
	}

	return allErrs
}

// validateWithoutCapabilities validates a machine image version when capabilities are not defined
func validateWithoutCapabilities(version api.MachineImageVersion, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// When not using capabilities, image must be set directly
	if len(version.Image) == 0 {
		allErrs = append(allErrs, field.Required(jdxPath.Child("image"), "must provide an image"))
	}

	// When not using capabilities, capabilitySets must NOT be used
	if len(version.CapabilitySets) > 0 {
		allErrs = append(allErrs, field.Forbidden(jdxPath.Child("capabilitySets"),
			"must not be set as CloudProfile does not define capabilities. Use the image field directly."))
	}

	return allErrs
}

// validateMachineImageMapping validates that for each machine image there is a corresponding providerConfig entry.
func validateMachineImageMapping(coreMachineImages []core.MachineImage, machineImages []api.MachineImages, capabilitiesDefinitions []gardencorev1beta1.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	providerImages := NewProviderImagesContext(machineImages)

	// validate machine images
	for idxImage, machineImage := range coreMachineImages {
		if len(machineImage.Versions) == 0 {
			continue
		}
		machineImagePath := fldPath.Index(idxImage)

		// validate that for each machine image there is a corresponding cpConfig image
		if _, existsInConfig := providerImages.GetImage(machineImage.Name); !existsInConfig {
			allErrs = append(allErrs, field.Required(machineImagePath,
				fmt.Sprintf("must provide an image mapping for image %q in providerConfig", machineImage.Name)))
			continue
		}

		allErrs = append(allErrs, validateMachineImageVersionMapping(machineImage, providerImages, capabilitiesDefinitions, machineImagePath)...)
	}

	return allErrs
}

// validateMachineImageVersionMapping validates that versions in a machine image have corresponding mappings in the providerConfig.
func validateMachineImageVersionMapping(machineImage core.MachineImage, providerImages *gardenerutils.ImagesContext[api.MachineImages, api.MachineImageVersion], capabilitiesDefinitions []gardencorev1beta1.CapabilityDefinition, machineImagePath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// If no capabilities are defined, no version mapping validation is needed
	if len(capabilitiesDefinitions) == 0 {
		return allErrs
	}

	// Validate each version has proper mapping
	for idxVersion, version := range machineImage.Versions {
		machineImageVersionPath := machineImagePath.Child("versions").Index(idxVersion)

		// Check if version exists in provider config
		imageVersion, exists := providerImages.GetImageVersion(machineImage.Name, version.Version)
		if !exists {
			allErrs = append(allErrs, field.Required(machineImageVersionPath,
				fmt.Sprintf("machine image version %s@%s is not defined in the providerConfig",
					machineImage.Name, version.Version),
			))
			continue // Skip further validation if version doesn't exist
		}

		// Validate capability sets mapping
		allErrs = append(allErrs, validateCapabilitySetMapping(machineImage.Name, version, imageVersion, capabilitiesDefinitions, machineImageVersionPath)...)
	}

	return allErrs
}

// validateCapabilitySetMapping validates that each capability set in a version has a corresponding mapping
func validateCapabilitySetMapping(imageName string, version core.MachineImageVersion, imageVersion api.MachineImageVersion, capabilitiesDefinitions []gardencorev1beta1.CapabilityDefinition, machineImageVersionPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var v1beta1Version gardencorev1beta1.MachineImageVersion
	if err := coreapi.Scheme.Convert(&version, &v1beta1Version, nil); err != nil {
		return append(allErrs, field.InternalError(machineImageVersionPath, err))
	}
	coreDefaultedCapabilitySets := v1beta1helper.GetCapabilitySetsWithAppliedDefaults(v1beta1Version.CapabilitySets, capabilitiesDefinitions)

	for idxCapability, coreDefaultedCapabilitySet := range coreDefaultedCapabilitySets {
		isFound := false
		// search for the corresponding imageVersion.CapabilitySet
		for _, providerCapabilitySet := range imageVersion.CapabilitySets {
			providerDefaultedCapabilities := v1beta1helper.GetCapabilitiesWithAppliedDefaults(providerCapabilitySet.Capabilities, capabilitiesDefinitions)
			if v1beta1helper.AreCapabilitiesEqual(coreDefaultedCapabilitySet.Capabilities, providerDefaultedCapabilities) {
				isFound = true
				break
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Required(machineImageVersionPath.Child("capabilitySets").Index(idxCapability),
				fmt.Sprintf("missing providerConfig mapping for machine image version %s@%s and capabilitySet %v",
					imageName, version.Version, coreDefaultedCapabilitySet.Capabilities)))
		}
	}

	return allErrs
}

// NewProviderImagesContext creates a new ImagesContext for provider images.
func NewProviderImagesContext(providerImages []api.MachineImages) *gardenerutils.ImagesContext[api.MachineImages, api.MachineImageVersion] {
	return gardenerutils.NewImagesContext(
		utils.CreateMapFromSlice(providerImages, func(mi api.MachineImages) string { return mi.Name }),
		func(mi api.MachineImages) map[string]api.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v api.MachineImageVersion) string { return v.Version })
		},
	)
}
