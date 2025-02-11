// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils"
	utilcore "github.com/gardener/gardener/pkg/utils/validation/gardener/core"
)

var (
	availableUpdateStrategiesForMachineImage = sets.New(
		string(core.UpdateStrategyPatch),
		string(core.UpdateStrategyMinor),
		string(core.UpdateStrategyMajor),
	)
)

// ValidateCloudProfile validates a CloudProfile object.
func ValidateCloudProfile(cloudProfile *core.CloudProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&cloudProfile.ObjectMeta, false, ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateCloudProfileSpec(&cloudProfile.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateCloudProfileUpdate validates a CloudProfile object before an update.
func ValidateCloudProfileUpdate(newProfile, oldProfile *core.CloudProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newProfile.ObjectMeta, &oldProfile.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateCloudProfile(newProfile)...)
	allErrs = append(allErrs, ValidateCloudProfileSpecUpdate(&newProfile.Spec, &oldProfile.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateCloudProfileSpec validates the specification of a CloudProfile object.
func ValidateCloudProfileSpec(spec *core.CloudProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must provide a provider type"))
	}

	// capabilitiesDefinition is used in the validate-functions for machineTypes and machineImages
	//   nil: the architecture field is required --> capabilities forbidden
	//   defined: the architecture field is forbidden --> capabilities required
	if utilfeature.DefaultFeatureGate.Enabled(features.CloudProfileCapabilities) {
		// If the feature gate is enabled and capabilitiesDefinition is set, it will be evaluated.
		// The capabilities and architecture fields cannot be set at the same time.
		errList := ValidateCapabilitiesDefinition(spec.CapabilitiesDefinition, fldPath.Child("capabilitiesDefinition"))
		if errList != nil {
			allErrs = append(allErrs, errList...)
		}
	} else {
		// If the feature gate is disabled, the capabilitiesDefinition must not be set.
		if spec.CapabilitiesDefinition.HasEntries() {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("capabilitiesDefinition"), "must not be defined as the CloudProfile Capabilities Feature is disabled."))
		}
	}

	allErrs = append(allErrs, ValidateCloudProfileMachineImages(spec.MachineImages, spec.CapabilitiesDefinition, fldPath.Child("machineImages"))...)
	allErrs = append(allErrs, validateCloudProfileMachineTypes(spec.MachineTypes, spec.CapabilitiesDefinition, fldPath.Child("machineTypes"))...)

	allErrs = append(allErrs, validateCloudProfileKubernetesSettings(spec.Kubernetes, fldPath.Child("kubernetes"))...)
	allErrs = append(allErrs, validateVolumeTypes(spec.VolumeTypes, fldPath.Child("volumeTypes"))...)
	allErrs = append(allErrs, validateCloudProfileRegions(spec.Regions, fldPath.Child("regions"))...)
	allErrs = append(allErrs, validateCloudProfileBastion(spec, fldPath.Child("bastion"))...)
	allErrs = append(allErrs, validateCloudProfileLimits(spec.Limits, fldPath.Child("limits"))...)
	if spec.SeedSelector != nil {
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(&spec.SeedSelector.LabelSelector, metav1validation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: true}, fldPath.Child("seedSelector"))...)
	}

	if spec.CABundle != nil {
		_, err := utils.DecodeCertificate([]byte(*(spec.CABundle)))
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("caBundle"), *(spec.CABundle), "caBundle is not a valid PEM-encoded certificate"))
		}
	}

	return allErrs
}

// ValidateCloudProfileSpecUpdate validates a CloudProfileSpec before an update.
func ValidateCloudProfileSpecUpdate(newSpec, oldSpec *core.CloudProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateCloudProfileLimitsUpdate(newSpec.Limits, oldSpec.Limits, fldPath.Child("limits"))...)

	return allErrs
}

func validateCloudProfileKubernetesSettings(kubernetes core.KubernetesSettings, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(kubernetes.Versions) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("versions"), "must provide at least one Kubernetes version"))
	}
	latestKubernetesVersion, _, err := helper.DetermineLatestExpirableVersion(kubernetes.Versions, false)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("versions"), latestKubernetesVersion.Version, "failed to determine the latest kubernetes version from the cloud profile"))
	}
	if latestKubernetesVersion.ExpirationDate != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("versions[]").Child("expirationDate"), latestKubernetesVersion.ExpirationDate, fmt.Sprintf("expiration date of latest kubernetes version ('%s') must not be set", latestKubernetesVersion.Version)))
	}

	allErrs = append(allErrs, validateKubernetesVersions(kubernetes.Versions, fldPath)...)

	for i, version := range kubernetes.Versions {
		idxPath := fldPath.Child("versions").Index(i)
		allErrs = append(allErrs, validateSupportedVersionsConfiguration(version, kubernetes.Versions, idxPath)...)
	}

	return allErrs
}

func validateSupportedVersionsConfiguration(version core.ExpirableVersion, allVersions []core.ExpirableVersion, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if version.Classification != nil && *version.Classification == core.ClassificationSupported {
		currentSemVer, err := semver.NewVersion(version.Version)
		if err != nil {
			// check is already performed by caller, avoid duplicate error
			return allErrs
		}

		filteredVersions, err := helper.FindVersionsWithSameMajorMinor(helper.FilterVersionsWithClassification(allVersions, core.ClassificationSupported), *currentSemVer)
		if err != nil {
			// check is already performed by caller, avoid duplicate error
			return allErrs
		}

		// do not allow adding multiple supported versions per minor version
		if len(filteredVersions) > 0 {
			allErrs = append(allErrs, field.Forbidden(fldPath, fmt.Sprintf("unable to add version %q with classification %q. Only one %q version is allowed per minor version", version.Version, core.ClassificationSupported, core.ClassificationSupported)))
		}
	}

	return allErrs
}

func validateCloudProfileMachineTypes(machineTypes []core.MachineType, capabilitiesDefinition core.Capabilities, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineTypes) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine type"))
	}
	allErrs = append(allErrs, validateMachineTypes(machineTypes, fldPath)...)
	allErrs = append(allErrs, validateMachineTypesCapabilities(machineTypes, capabilitiesDefinition, fldPath)...)

	return allErrs
}

// ValidateCloudProfileMachineImages validates the machine images of a CloudProfile object.
func ValidateCloudProfileMachineImages(machineImages []core.MachineImage, capabilitiesDefinition core.Capabilities, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineImages) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine image"))
	}

	allErrs = append(allErrs, ValidateMachineImages(machineImages, fldPath, false)...)
	allErrs = append(allErrs, ValidateMachineImageCapabilities(machineImages, capabilitiesDefinition, fldPath)...)
	for i, image := range machineImages {
		idxPath := fldPath.Index(i)

		if image.UpdateStrategy == nil {
			allErrs = append(allErrs, field.Required(idxPath.Child("updateStrategy"), "must provide an update strategy"))
		}

		for index, machineVersion := range image.Versions {
			versionsPath := idxPath.Child("versions").Index(index)
			allErrs = append(allErrs, validateContainerRuntimesInterfaces(machineVersion.CRI, versionsPath.Child("cri"))...)
			allErrs = append(allErrs, validateSupportedVersionsConfiguration(machineVersion.ExpirableVersion, helper.ToExpirableVersions(image.Versions), versionsPath)...)

			if !capabilitiesDefinition.HasEntries() && len(machineVersion.Architectures) == 0 {
				allErrs = append(allErrs, field.Required(versionsPath.Child("architectures"), "must provide at least one architecture"))
			}
		}
	}

	return allErrs
}

func validateContainerRuntimesInterfaces(cris []core.CRI, fldPath *field.Path) field.ErrorList {
	var (
		allErrs      = field.ErrorList{}
		duplicateCRI = sets.Set[string]{}
	)

	if len(cris) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one supported container runtime"))
		return allErrs
	}

	for i, cri := range cris {
		criPath := fldPath.Index(i)
		if duplicateCRI.Has(string(cri.Name)) {
			allErrs = append(allErrs, field.Duplicate(criPath, cri.Name))
		}
		duplicateCRI.Insert(string(cri.Name))

		if !availableWorkerCRINames.Has(string(cri.Name)) {
			allErrs = append(allErrs, field.NotSupported(criPath.Child("name"), string(cri.Name), sets.List(availableWorkerCRINames)))
		}
		allErrs = append(allErrs, validateContainerRuntimes(cri.ContainerRuntimes, criPath.Child("containerRuntimes"))...)
	}

	return allErrs
}

func validateContainerRuntimes(containerRuntimes []core.ContainerRuntime, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	duplicateCR := sets.Set[string]{}

	for i, cr := range containerRuntimes {
		if duplicateCR.Has(cr.Type) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("type"), cr.Type))
		}
		duplicateCR.Insert(cr.Type)
	}

	return allErrs
}

func validateCloudProfileRegions(regions []core.Region, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(regions) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one region"))
	}

	regionsFound := sets.New[string]()
	for i, region := range regions {
		idxPath := fldPath.Index(i)
		namePath := idxPath.Child("name")
		zonesPath := idxPath.Child("zones")
		labelsPath := idxPath.Child("labels")

		if len(region.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a region name"))
		} else if regionsFound.Has(region.Name) {
			allErrs = append(allErrs, field.Duplicate(namePath, region.Name))
		} else {
			regionsFound.Insert(region.Name)
		}

		zonesFound := sets.New[string]()
		for j, zone := range region.Zones {
			namePath := zonesPath.Index(j).Child("name")
			if len(zone.Name) == 0 {
				allErrs = append(allErrs, field.Required(namePath, "zone name cannot be empty"))
			} else if zonesFound.Has(zone.Name) {
				allErrs = append(allErrs, field.Duplicate(namePath, zone.Name))
			} else {
				zonesFound.Insert(zone.Name)
			}
		}

		allErrs = append(allErrs, metav1validation.ValidateLabels(region.Labels, labelsPath)...)
	}

	return allErrs
}

func validateCloudProfileBastion(spec *core.CloudProfileSpec, fldPath *field.Path) field.ErrorList {
	var (
		allErrs     field.ErrorList
		machineArch *string
	)

	if spec.Bastion == nil {
		return allErrs
	}

	if spec.Bastion.MachineType == nil && spec.Bastion.MachineImage == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, spec.Bastion, "bastion section needs a machine type or machine image"))
	}

	if spec.Bastion.MachineType != nil {
		var validationErrors field.ErrorList
		machineArch, validationErrors = validateBastionMachineType(spec.Bastion.MachineType, spec.MachineTypes, fldPath.Child("machineType"))
		allErrs = append(allErrs, validationErrors...)
	}

	if spec.Bastion.MachineImage != nil {
		allErrs = append(allErrs, validateBastionImage(spec.Bastion.MachineImage, spec.MachineImages, machineArch, fldPath.Child("machineImage"))...)
	}

	return allErrs
}

func validateBastionMachineType(bastionMachineType *core.BastionMachineType, machineTypes []core.MachineType, fldPath *field.Path) (*string, field.ErrorList) {
	machineIndex := slices.IndexFunc(machineTypes, func(machineType core.MachineType) bool {
		return machineType.Name == bastionMachineType.Name
	})

	if machineIndex == -1 {
		return nil, field.ErrorList{field.Invalid(fldPath.Child("name"), bastionMachineType.Name, "machine type not found in spec.machineTypes")}
	}

	return machineTypes[machineIndex].Architecture, nil
}

func validateBastionImage(bastionImage *core.BastionMachineImage, machineImages []core.MachineImage, machineArch *string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	namePath := fldPath.Child("name")

	imageIndex := slices.IndexFunc(machineImages, func(image core.MachineImage) bool {
		return image.Name == bastionImage.Name
	})

	if imageIndex == -1 {
		return append(allErrs, field.Invalid(namePath, bastionImage.Name, "image name not found in spec.machineImages"))
	}

	imageVersions := machineImages[imageIndex].Versions

	if bastionImage.Version == nil {
		allErrs = append(allErrs, checkImageSupport(bastionImage.Name, imageVersions, machineArch, namePath)...)
	} else {
		versionPath := fldPath.Child("version")

		versionIndex := slices.IndexFunc(imageVersions, func(version core.MachineImageVersion) bool {
			return version.Version == *bastionImage.Version
		})

		if versionIndex == -1 {
			return append(allErrs, field.Invalid(versionPath, bastionImage.Version, "image version not found in spec.machineImages"))
		}

		imageVersion := []core.MachineImageVersion{imageVersions[versionIndex]}
		allErrs = append(allErrs, checkImageSupport(bastionImage.Name, imageVersion, machineArch, versionPath)...)
	}

	return allErrs
}

func checkImageSupport(bastionImageName string, imageVersions []core.MachineImageVersion, machineArch *string, fldPath *field.Path) field.ErrorList {
	for _, version := range imageVersions {
		archSupported := false
		validClassification := false

		if machineArch != nil && slices.Contains(version.Architectures, *machineArch) {
			archSupported = true
		}
		// any arch is supported in case machineArch is nil
		if machineArch == nil && len(version.Architectures) > 0 {
			archSupported = true
		}
		if version.Classification != nil && *version.Classification == core.ClassificationSupported {
			validClassification = true
		}
		if archSupported && validClassification {
			return nil
		}
	}

	return field.ErrorList{field.Invalid(fldPath, bastionImageName,
		fmt.Sprintf("no image with classification supported and arch %q found", ptr.Deref(machineArch, "<nil>")))}
}

func validateCloudProfileLimits(limits *core.Limits, fldPath *field.Path) field.ErrorList {
	if limits == nil {
		return nil
	}

	var allErrs field.ErrorList

	if maxNodesTotal := limits.MaxNodesTotal; maxNodesTotal != nil && *maxNodesTotal <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxNodesTotal"), *maxNodesTotal, "maxNodesTotal must be greater than 0"))
	}

	return allErrs
}

func validateCloudProfileLimitsUpdate(newLimits, oldLimits *core.Limits, fldPath *field.Path) field.ErrorList {
	if newLimits == nil || oldLimits == nil {
		// adding and removing limits is allowed
		return nil
	}

	var allErrs field.ErrorList

	var (
		newMaxNodesTotal = newLimits.MaxNodesTotal
		oldMaxNodesTotal = oldLimits.MaxNodesTotal
	)
	if newMaxNodesTotal != nil && oldMaxNodesTotal != nil && *newMaxNodesTotal < *oldMaxNodesTotal {
		// adding, removing, and increasing maxNodesTotal is allowed, but not decreasing
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxNodesTotal"), *newMaxNodesTotal, "maxNodesTotal cannot be decreased"))
	}

	return allErrs
}

// ValidateCapabilitiesDefinition validates the capabilitiesDefinition of a cloudProfile, ensures that the architecture is set and that no capability is empty
func ValidateCapabilitiesDefinition(capabilitiesDefinition core.Capabilities, path *field.Path) field.ErrorList {
	var errList field.ErrorList

	// during the transition period to capabilities, capabilitiesDefinition is optional thus the empty definition is allowed
	// this check must be removed after capabilitiesDefinition is required
	if !capabilitiesDefinition.HasEntries() {
		return errList
	}

	// architecture is a required capability
	val, ok := capabilitiesDefinition[v1beta1constants.ArchitectureKey]
	if ok {
		errList = append(errList, validateMachineImageVersionArchitecture(val.Values, path.Child(v1beta1constants.ArchitectureKey))...)
	} else {
		errList = append(errList, field.Required(path.Child(v1beta1constants.ArchitectureKey),
			"allowed architectures are: "+strings.Join(v1beta1constants.ValidArchitectures, ", ")))
	}

	// No empty capabilities allowed
	for capabilityName, capabilityValues := range capabilitiesDefinition {
		if len(capabilityName) == 0 {
			errList = append(errList, field.Invalid(path, "", "empty capability name is not allowed"))
		}
		if len(capabilityValues.Values) == 0 {
			errList = append(errList, field.Required(path.Child(string(capabilityName)), "must not be empty"))
		} else {
			for _, capabilityValue := range capabilityValues.Values {
				if len(capabilityValue) == 0 {
					errList = append(errList, field.Invalid(path.Child(string(capabilityName)), capabilityValues, "must not contain empty capability values"))
				}
			}
		}
	}
	return errList
}

// ValidateMachineImageCapabilities validates the given list of machine images for valid capabilities and architecture values.
func ValidateMachineImageCapabilities(machineImages []core.MachineImage, capabilitiesDefinition core.Capabilities, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, image := range machineImages {
		idxPath := fldPath.Index(i)
		for index, machineVersion := range image.Versions {
			versionsPath := idxPath.Child("versions").Index(index)
			if capabilitiesDefinition.HasEntries() {
				allErrs = append(allErrs, validateMachineImageVersionCapabilities(machineVersion, capabilitiesDefinition, versionsPath)...)
			} else {
				allErrs = append(allErrs, validateMachineImageVersionArchitecture(machineVersion.Architectures, versionsPath.Child(v1beta1constants.ArchitectureKey))...)
				if len(machineVersion.CapabilitiesSet) > 0 {
					allErrs = append(allErrs, field.Forbidden(versionsPath.Child("capabilitiesSet"), "must not provide CapabilitiesSet when no capabilitiesDefinition is defined"))
				}
			}
		}
	}

	return allErrs
}

func validateMachineImageVersionCapabilities(machineImageVersion core.MachineImageVersion, capabilitiesDefinition core.Capabilities, path *field.Path) field.ErrorList {
	errList := field.ErrorList{}

	if machineImageVersion.Architectures != nil {
		errList = append(errList, field.Forbidden(path.Child("architectures"), "must not be set when capabilities are defined"))
	}

	capabilitiesSet, unmarshalErrorList := utilcore.UnmarshalCapabilitiesSet(machineImageVersion.CapabilitiesSet, path)
	if unmarshalErrorList != nil {
		return append(errList, unmarshalErrorList...)
	}

	if len(capabilitiesDefinition[v1beta1constants.ArchitectureKey].Values) > 1 && len(capabilitiesSet) == 0 {
		errList = append(errList, field.Required(path.Child("capabilitiesSet"), "must be provided when multiple architectures are supported in the cloud profile"))
	}

	for i, capabilities := range capabilitiesSet {
		errList = append(errList, utilcore.ValidateCapabilitiesAgainstDefinition(capabilities, capabilitiesDefinition, path.Child("capabilitiesSet").Index(i))...)
		errList = append(errList, validateArchitectureCapability(capabilities, capabilitiesDefinition, path.Child("capabilitiesSet").Index(i))...)
	}

	return errList
}

// validateMachineTypesCapabilities validates the given list of machine types for valid capabilities and architecture values.
func validateMachineTypesCapabilities(machineTypes []core.MachineType, capabilitiesDefinition core.Capabilities, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, machineType := range machineTypes {
		idxPath := fldPath.Index(i)
		archPath := idxPath.Child(v1beta1constants.ArchitectureKey)

		if capabilitiesDefinition.HasEntries() {
			allErrs = append(allErrs, ValidateMachineTypeCapabilities(machineType, capabilitiesDefinition, idxPath)...)
		} else {
			allErrs = append(allErrs, validateMachineTypeArchitecture(machineType.Architecture, archPath)...)
			if machineType.Capabilities != nil {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("capabilities"), "must not provide capabilities when no capabilitiesDefinition is defined"))
			}
		}
	}
	return allErrs
}

// ValidateMachineTypeCapabilities validates the capabilities of a machineType, ensures that the architecture is not set and that no capability is empty
func ValidateMachineTypeCapabilities(machineType core.MachineType, capabilitiesDefinition core.Capabilities, path *field.Path) field.ErrorList {
	errList := field.ErrorList{}

	errList = append(errList, utilcore.ValidateCapabilitiesAgainstDefinition(machineType.Capabilities, capabilitiesDefinition, path.Child("capabilities"))...)
	errList = append(errList, validateArchitectureCapability(machineType.Capabilities, capabilitiesDefinition, path.Child("capabilities"))...)

	if len(ptr.Deref(machineType.Architecture, "")) > 0 {
		errList = append(errList, field.Forbidden(path.Child(v1beta1constants.ArchitectureKey), "must not be set when capabilities are defined"))
	}

	return errList
}

func validateArchitectureCapability(capabilities core.Capabilities, capabilitiesDefinition core.Capabilities, path *field.Path) field.ErrorList {
	errList := field.ErrorList{}

	// if there are multiple values for architecture, the architecture capability must be set and must contain exactly one value
	allowedArchitectures := capabilitiesDefinition[v1beta1constants.ArchitectureKey].Values

	if len(allowedArchitectures) > 1 {
		if value, ok := capabilities[v1beta1constants.ArchitectureKey]; !ok {
			errList = append(errList, field.Required(path.Child(v1beta1constants.ArchitectureKey), fmt.Sprintf("multiple architectures are supported in the cloud profile. So it must be defined and contain exactly one of: %+v", allowedArchitectures)))
		} else if len(value.Values) != 1 {
			errList = append(errList, field.Invalid(path.Child(v1beta1constants.ArchitectureKey), value.Values, fmt.Sprintf("must contain exactly one of: %+v", allowedArchitectures)))
		}
	}
	return errList
}
