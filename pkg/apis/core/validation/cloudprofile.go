// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	kubernetescorevalidation "github.com/gardener/gardener/pkg/utils/validation/kubernetes/core"
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
	allErrs = append(allErrs, ValidateCloudProfileSpecUpdate(&newProfile.Spec, &oldProfile.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateCloudProfile(newProfile)...)

	return allErrs
}

// ValidateCloudProfileSpecUpdate validates the spec update of a CloudProfile
func ValidateCloudProfileSpecUpdate(_, _ *core.CloudProfileSpec, _ *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

// ValidateCloudProfileSpec validates the specification of a CloudProfile object.
func ValidateCloudProfileSpec(spec *core.CloudProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must provide a provider type"))
	}

	allErrs = append(allErrs, validateKubernetesSettings(spec.Kubernetes, fldPath.Child("kubernetes"))...)
	allErrs = append(allErrs, validateMachineImagesConfiguration(spec.MachineImages, fldPath.Child("machineImages"))...)
	allErrs = append(allErrs, validateMandatoryMachineTypes(spec.MachineTypes, fldPath.Child("machineTypes"))...)
	allErrs = append(allErrs, validateVolumeTypes(spec.VolumeTypes, fldPath.Child("volumeTypes"))...)
	allErrs = append(allErrs, validateRegions(spec.Regions, fldPath.Child("regions"))...)
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

func validateKubernetesSettings(kubernetes core.KubernetesSettings, fldPath *field.Path) field.ErrorList {
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

	allErrs = append(allErrs, ValidateKubernetesVersions(kubernetes.Versions, fldPath)...)

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

func validateMandatoryMachineTypes(machineTypes []core.MachineType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineTypes) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine type"))
	}
	allErrs = append(allErrs, ValidateMachineTypes(machineTypes, fldPath)...)

	return allErrs
}

func validateMachineTypeStorage(storage core.MachineTypeStorage, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if storage.StorageSize == nil && storage.MinSize == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, storage, `must either configure "size" or "minSize"`))
		return allErrs
	}

	if storage.StorageSize != nil && storage.MinSize != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, storage, `not allowed to configure both "size" and "minSize"`))
		return allErrs
	}

	if storage.StorageSize != nil {
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("size", *storage.StorageSize, fldPath.Child("size"))...)
	}

	if storage.MinSize != nil {
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("minSize", *storage.MinSize, fldPath.Child("minSize"))...)
	}

	return allErrs
}

func validateMachineImagesConfiguration(machineImages []core.MachineImage, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineImages) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine image"))
	}

	allErrs = append(allErrs, ValidateMachineImages(machineImages, fldPath)...)

	for i, image := range machineImages {
		idxPath := fldPath.Index(i)
		for index, machineVersion := range image.Versions {
			versionsPath := idxPath.Child("versions").Index(index)
			allErrs = append(allErrs, validateContainerRuntimesInterfaces(machineVersion.CRI, versionsPath.Child("cri"))...)
			allErrs = append(allErrs, validateSupportedVersionsConfiguration(machineVersion.ExpirableVersion, helper.ToExpirableVersions(image.Versions), versionsPath)...)
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

func validateMachineImageVersionArchitecture(archs []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, arch := range archs {
		if !slices.Contains(v1beta1constants.ValidArchitectures, arch) {
			allErrs = append(allErrs, field.NotSupported(fldPath, arch, v1beta1constants.ValidArchitectures))
		}
	}

	return allErrs
}

func validateMachineTypeArchitecture(arch *string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if !slices.Contains(v1beta1constants.ValidArchitectures, *arch) {
		allErrs = append(allErrs, field.NotSupported(fldPath, *arch, v1beta1constants.ValidArchitectures))
	}

	return allErrs
}

func validateVolumeTypes(volumeTypes []core.VolumeType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	names := make(map[string]struct{}, len(volumeTypes))

	for i, volumeType := range volumeTypes {
		idxPath := fldPath.Index(i)

		namePath := idxPath.Child("name")
		if len(volumeType.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a name"))
		}

		if _, ok := names[volumeType.Name]; ok {
			allErrs = append(allErrs, field.Duplicate(namePath, volumeType.Name))
			break
		}
		names[volumeType.Name] = struct{}{}

		if len(volumeType.Class) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("class"), "must provide a class"))
		}

		if volumeType.MinSize != nil {
			allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("minSize", *volumeType.MinSize, idxPath.Child("minSize"))...)
		}
	}

	return allErrs
}

func validateRegions(regions []core.Region, fldPath *field.Path) field.ErrorList {
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
