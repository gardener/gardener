// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	var (
		allErrs             = field.ErrorList{}
		machineCapabilities = helper.CapabilityDefinitionsToCapabilities(spec.MachineCapabilities)
	)

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must provide a provider type"))
	}

	allErrs = append(allErrs, validateCapabilityDefinitions(spec.MachineCapabilities, fldPath.Child("machineCapabilities"))...)
	allErrs = append(allErrs, validateCloudProfileKubernetesSettings(spec.Kubernetes, fldPath.Child("kubernetes"))...)
	allErrs = append(allErrs, ValidateCloudProfileMachineImages(spec.MachineImages, machineCapabilities, fldPath.Child("machineImages"))...)
	allErrs = append(allErrs, validateCloudProfileMachineTypes(spec.MachineTypes, machineCapabilities, fldPath.Child("machineTypes"))...)
	allErrs = append(allErrs, validateVolumeTypes(spec.VolumeTypes, fldPath.Child("volumeTypes"))...)
	allErrs = append(allErrs, validateCloudProfileRegions(spec.Regions, fldPath.Child("regions"))...)
	allErrs = append(allErrs, validateCloudProfileBastion(spec, fldPath.Child("bastion"))...)
	allErrs = append(allErrs, validateCloudProfileLimits(spec.Limits, fldPath.Child("limits"))...)
	if spec.SeedSelector != nil {
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(&spec.SeedSelector.LabelSelector, metav1validation.LabelSelectorValidationOptions{}, fldPath.Child("seedSelector"))...)
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

	// TODO(LucaBernstein): Check whether this behavior should be corrected (i.e. changed) in a later GEP-32-PR.
	//  The current behavior for nil classifications is treated differently across the codebase.
	if version.Classification != nil && helper.CurrentLifecycleClassification(version) == core.ClassificationSupported {
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

func validateCloudProfileMachineTypes(machineTypes []core.MachineType, capabilities core.Capabilities, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineTypes) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine type"))
	}
	allErrs = append(allErrs, validateMachineTypes(machineTypes, capabilities, fldPath)...)

	for i, machineType := range machineTypes {
		if ptr.Deref(machineType.Architecture, "") == "" && len(capabilities) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Index(i).Child("architecture"), "must provide an architecture"))
		}
		if len(capabilities) == 0 && len(machineType.Capabilities) > 0 {
			allErrs = append(allErrs, field.Forbidden(fldPath.Index(i).Child("capabilities"), "must not provide capabilities without global definition"))
		}
	}

	return allErrs
}

// ValidateCloudProfileMachineImages validates the machine images of a CloudProfile object.
func ValidateCloudProfileMachineImages(machineImages []core.MachineImage, capabilities core.Capabilities, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineImages) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine image"))
	}

	allErrs = append(allErrs, ValidateMachineImages(machineImages, capabilities, fldPath, false)...)

	for i, image := range machineImages {
		idxPath := fldPath.Index(i)

		if image.UpdateStrategy == nil {
			allErrs = append(allErrs, field.Required(idxPath.Child("updateStrategy"), "must provide an update strategy"))
		}

		for index, machineVersion := range image.Versions {
			versionsPath := idxPath.Child("versions").Index(index)
			allErrs = append(allErrs, validateContainerRuntimesInterfaces(machineVersion.CRI, versionsPath.Child("cri"))...)
			allErrs = append(allErrs, validateSupportedVersionsConfiguration(machineVersion.ExpirableVersion, helper.ToExpirableVersions(image.Versions), versionsPath)...)

			if len(capabilities) == 0 {
				if len(machineVersion.Architectures) == 0 {
					allErrs = append(allErrs, field.Required(versionsPath.Child("architectures"), "must provide at least one architecture"))
				}
				if len(machineVersion.CapabilityFlavors) > 0 {
					allErrs = append(allErrs, field.Forbidden(versionsPath.Child("capabilityFlavors"), "must not provide capabilities without global definition"))
				}
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
		if errs := validateUnprefixedQualifiedName(cr.Type); len(errs) != 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("type"), cr.Type, fmt.Sprintf("container runtime type must be a qualified name: %v", errs)))
		}
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
		allErrs = append(allErrs, validateBastionImage(spec.Bastion.MachineImage, spec.MachineImages, helper.CapabilityDefinitionsToCapabilities(spec.MachineCapabilities), machineArch, fldPath.Child("machineImage"))...)
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

	return ptr.To(machineTypes[machineIndex].GetArchitecture()), nil
}

func validateBastionImage(bastionImage *core.BastionMachineImage, machineImages []core.MachineImage, capabilities core.Capabilities, machineArch *string, fldPath *field.Path) field.ErrorList {
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
		allErrs = append(allErrs, checkImageSupport(bastionImage.Name, imageVersions, capabilities, machineArch, namePath)...)
	} else {
		versionPath := fldPath.Child("version")

		versionIndex := slices.IndexFunc(imageVersions, func(version core.MachineImageVersion) bool {
			return version.Version == *bastionImage.Version
		})

		if versionIndex == -1 {
			return append(allErrs, field.Invalid(versionPath, bastionImage.Version, "image version not found in spec.machineImages"))
		}

		imageVersion := []core.MachineImageVersion{imageVersions[versionIndex]}
		allErrs = append(allErrs, checkImageSupport(bastionImage.Name, imageVersion, capabilities, machineArch, versionPath)...)
	}

	return allErrs
}

func checkImageSupport(bastionImageName string, imageVersions []core.MachineImageVersion, capabilities core.Capabilities, machineArch *string, fldPath *field.Path) field.ErrorList {
	for _, version := range imageVersions {
		archSupported := false
		validClassification := false

		// any arch is supported in case machineArch is nil
		if machineArch == nil || version.SupportsArchitecture(capabilities, *machineArch) {
			archSupported = true
		}

		// TODO(LucaBernstein): Check whether this behavior should be corrected (i.e. changed) in a later GEP-32-PR.
		//  The current behavior for nil classifications is treated differently across the codebase.
		if version.Classification != nil && helper.CurrentLifecycleClassification(version.ExpirableVersion) == core.ClassificationSupported {
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
	if HasDecreasedMaxNodesTotal(newMaxNodesTotal, oldMaxNodesTotal) {
		// adding, removing, and increasing maxNodesTotal is allowed, but not decreasing
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxNodesTotal"), *newMaxNodesTotal, "maxNodesTotal cannot be decreased"))
	}

	return allErrs
}

// HasDecreasedMaxNodesTotal checks whether the new maxNodesTotal has been decreased.
func HasDecreasedMaxNodesTotal(newMaxNodesTotal, oldMaxNodesTotal *int32) bool {
	return newMaxNodesTotal != nil && oldMaxNodesTotal != nil && *newMaxNodesTotal < *oldMaxNodesTotal
}

func validateCapabilityDefinitions(capabilityDefinitions []core.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(capabilityDefinitions) == 0 {
		return allErrs
	}

	if !utilfeature.DefaultFeatureGate.Enabled(features.CloudProfileCapabilities) {
		allErrs = append(allErrs, field.Forbidden(fldPath, "machineCapabilities are not allowed with disabled CloudProfileCapabilities feature gate"))
	}

	capabilityMap := make(core.Capabilities, len(capabilityDefinitions))
	for idx, capability := range capabilityDefinitions {
		capabilityDefinitionFieldPath := fldPath.Index(idx)
		if _, exists := capabilityMap[capability.Name]; exists {
			allErrs = append(allErrs, field.Duplicate(capabilityDefinitionFieldPath.Child("name"), capability.Name))
		}
		capabilityMap[capability.Name] = capability.Values
	}

	// The 'architecture' capability definition is required.
	// It corresponds to the older, dedicated 'architecture' fields in the CloudProfile.
	val, ok := capabilityMap[v1beta1constants.ArchitectureName]
	if !ok {
		allErrs = append(allErrs, field.Required(fldPath.Child(v1beta1constants.ArchitectureName), "architecture capability is required"))
	} else {
		for _, v := range val {
			if !slices.Contains(v1beta1constants.ValidArchitectures, v) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child(v1beta1constants.ArchitectureName), v, "allowed architectures are: "+strings.Join(v1beta1constants.ValidArchitectures, ", ")))
			}
		}
	}

	// CapabilityDefinition keys defined must not be empty.
	for key, value := range capabilityMap {
		if key == "" {
			allErrs = append(allErrs, field.Required(fldPath, "capability keys must not be empty"))
		} else if errs := validateUnprefixedQualifiedName(key); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath, key, "capability key must be qualified name: "+strings.Join(errs, ", ")))
		}
		if len(value) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child(key), "capability values must not be empty"))
		}
		for i, v := range value {
			if v == "" {
				allErrs = append(allErrs, field.Required(fldPath.Child(key).Index(i), "capability values must not be empty"))
			} else if errs := validateUnprefixedQualifiedName(v); len(errs) > 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child(key).Index(i), v, "capability value must be qualified name: "+strings.Join(errs, ", ")))
			}
		}
	}

	return allErrs
}
