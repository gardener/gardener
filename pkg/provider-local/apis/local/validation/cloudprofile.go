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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ValidateCloudProfileConfig validates a CloudProfileConfig object.
func ValidateCloudProfileConfig(cpConfig *api.CloudProfileConfig, machineImages []core.MachineImage, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	machineImagesPath := fldPath.Child("machineImages")

	// Validate machine images section
	allErrs = append(allErrs, validateMachineImages(cpConfig.MachineImages, capabilityDefinitions, machineImagesPath)...)
	if len(allErrs) > 0 {
		return allErrs
	}

	// Validate machine image mappings
	allErrs = append(allErrs, validateMachineImageMapping(machineImages, cpConfig.MachineImages, capabilityDefinitions, machineImagesPath)...)

	return allErrs
}

// validateMachineImages validates the machine images section of CloudProfileConfig
func validateMachineImages(machineImages []api.MachineImages, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure at least one machine image is provided
	if len(machineImages) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine image"))
		return allErrs
	}

	// Validate each machine image
	for i, machineImage := range machineImages {
		idxPath := fldPath.Index(i)
		allErrs = append(allErrs, validateMachineImage(machineImage, capabilityDefinitions, idxPath)...)
	}

	return allErrs
}

// validateMachineImage validates an individual machine image configuration
func validateMachineImage(machineImage api.MachineImages, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, idxPath *field.Path) field.ErrorList {
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
		allErrs = append(allErrs, validateMachineImageVersion(version, capabilityDefinitions, jdxPath)...)
	}

	return allErrs
}

// validateMachineImageVersion validates a specific machine image version
func validateMachineImageVersion(version api.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(version.Version) == 0 {
		allErrs = append(allErrs, field.Required(jdxPath.Child("version"), "must provide a version"))
	}

	// Different validation paths based on whether capabilities are defined
	if len(capabilityDefinitions) > 0 {
		allErrs = append(allErrs, validateWithCapabilities(version, capabilityDefinitions, jdxPath)...)
	} else {
		allErrs = append(allErrs, validateWithoutCapabilities(version, jdxPath)...)
	}

	return allErrs
}

// validateWithCapabilities validates a machine image version when capabilities are defined
func validateWithCapabilities(version api.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// When using capabilities, image must not be set directly
	if len(version.Image) > 0 {
		allErrs = append(allErrs, field.Forbidden(jdxPath.Child("image"),
			"must not be set as CloudProfile defines capabilities. Use capabilityFlavors[].image instead."))
	}

	// Validate each flavor's image and capabilities
	for k, imageFlavor := range version.CapabilityFlavors {
		kdxPath := jdxPath.Child("capabilityFlavors").Index(k)
		if len(imageFlavor.Image) == 0 {
			allErrs = append(allErrs, field.Required(kdxPath.Child("image"), "must provide an image"))
		}
		allErrs = append(allErrs, gardenerutils.ValidateCapabilities(imageFlavor.Capabilities, capabilityDefinitions, kdxPath.Child("capabilities"))...)
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

	// When not using capabilities, capabilityFlavors must NOT be used
	if len(version.CapabilityFlavors) > 0 {
		allErrs = append(allErrs, field.Forbidden(jdxPath.Child("capabilityFlavors"),
			"must not be set as CloudProfile does not define capabilities. Use the image field directly."))
	}

	return allErrs
}

// validateMachineImageMapping validates that for each machine image there is a corresponding providerConfig entry.
func validateMachineImageMapping(coreMachineImages []core.MachineImage, machineImages []api.MachineImages, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
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
			allErrs = append(allErrs, field.Required(machineImagePath, fmt.Sprintf("must provide an image mapping for image %q in providerConfig", machineImage.Name)))
			continue
		}

		allErrs = append(allErrs, validateMachineImageVersionMapping(machineImage, providerImages, capabilityDefinitions, machineImagePath)...)
	}

	return allErrs
}

// validateMachineImageVersionMapping validates that versions in a machine image have corresponding mappings in the providerConfig.
func validateMachineImageVersionMapping(machineImage core.MachineImage, providerImages *gardenerutils.ImagesContext[api.MachineImages, api.MachineImageVersion], capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, machineImagePath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// If no capabilities are defined, no version mapping validation is needed
	if len(capabilityDefinitions) == 0 {
		return allErrs
	}

	// Validate each version has proper mapping
	for idxVersion, version := range machineImage.Versions {
		machineImageVersionPath := machineImagePath.Child("versions").Index(idxVersion)

		// Check if version exists in provider config
		imageVersion, exists := providerImages.GetImageVersion(machineImage.Name, version.Version)
		if !exists {
			allErrs = append(allErrs, field.Required(machineImageVersionPath,
				fmt.Sprintf("machine image version %s@%s is not defined in the providerConfig", machineImage.Name, version.Version),
			))
			continue // Skip further validation if version doesn't exist
		}

		// Validate image version flavor mapping
		allErrs = append(allErrs, validateImageFlavorMapping(machineImage.Name, version, imageVersion, capabilityDefinitions, machineImageVersionPath)...)
	}

	return allErrs
}

// validateImageFlavorMapping validates that each flavor in a version has a corresponding mapping
func validateImageFlavorMapping(imageName string, version core.MachineImageVersion, imageVersion api.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, machineImageVersionPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var v1beta1Version gardencorev1beta1.MachineImageVersion
	if err := coreapi.Scheme.Convert(&version, &v1beta1Version, nil); err != nil {
		return append(allErrs, field.InternalError(machineImageVersionPath, err))
	}
	coreDefaultedImageFlavors := v1beta1helper.GetImageFlavorsWithAppliedDefaults(v1beta1Version.CapabilityFlavors, capabilityDefinitions)

	for idxCapability, coreDefaultedFlavor := range coreDefaultedImageFlavors {
		isFound := false
		// search for the corresponding imageVersion.MachineImageFlavor
		for _, providerFlavor := range imageVersion.CapabilityFlavors {
			providerDefaultedCapabilities := v1beta1helper.GetCapabilitiesWithAppliedDefaults(providerFlavor.Capabilities, capabilityDefinitions)
			if v1beta1helper.AreCapabilitiesEqual(coreDefaultedFlavor.Capabilities, providerDefaultedCapabilities) {
				isFound = true
				break
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Required(machineImageVersionPath.Child("capabilityFlavors").Index(idxCapability),
				fmt.Sprintf("missing providerConfig mapping for machine image version %s@%s and flavor %v",
					imageName, version.Version, coreDefaultedFlavor.Capabilities)))
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

// ValidateSupportedCapabilities validates that only supported capabilities and values are used.
// This function can be deleted once the dedicated architecture field on MachineType and MachineImageVersion is retired.
// This strict behavior is required to ensure automatic format conversion of namespaced CloudProfiles during the transition to machine capabilities.
func ValidateSupportedCapabilities(capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, child *field.Path) error {
	// During transition to machineCapability based cloud profiles only the following capability is allowed to be set: architecture
	allErrs := field.ErrorList{}
	for i, def := range capabilityDefinitions {
		idxPath := child.Index(i)
		if def.Name != v1beta1constants.ArchitectureName {
			allErrs = append(allErrs, field.NotSupported(idxPath.Child("name"), def.Name, []string{v1beta1constants.ArchitectureName}))
		}
		for j, value := range def.Values {
			jdxPath := idxPath.Child("values").Index(j)
			if value != v1beta1constants.ArchitectureAMD64 && value != v1beta1constants.ArchitectureARM64 {
				allErrs = append(allErrs, field.NotSupported(jdxPath, value, []string{v1beta1constants.ArchitectureAMD64, v1beta1constants.ArchitectureARM64}))
			}
		}
	}
	return allErrs.ToAggregate()
}
