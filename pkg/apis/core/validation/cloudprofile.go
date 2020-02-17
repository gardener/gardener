// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

import (
	"fmt"
	"regexp"
	"time"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	"github.com/gardener/gardener/pkg/utils"

	"github.com/Masterminds/semver"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
func ValidateCloudProfileSpecUpdate(new *core.CloudProfileSpec, old *core.CloudProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateCloudProfileVersionsUpdate(new.Kubernetes.Versions, old.Kubernetes.Versions, fldPath.Child("kubernetes", "versions"))...)

	for index, _ := range old.MachineImages {
		if len(new.MachineImages) - 1  >= index {
			allErrs = append(allErrs, validateCloudProfileVersionsUpdate(new.MachineImages[index].Versions, old.MachineImages[index].Versions, fldPath.Child("machineImages").Index(index).Child("versions"))...)
		}
	}

	return allErrs
}

// ValidateCloudProfileAddedVersions validates versions added to the CloudProfile
func ValidateCloudProfileAddedVersions(versions []core.ExpirableVersion, addedVersions map[int]core.ExpirableVersion, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for index, version := range addedVersions {
		// Do not allow adding addedVersions with expiration date in the past
		if  version.ExpirationDate != nil && version.ExpirationDate.Time.UTC().Before(time.Now()) {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(index), version.Version, fmt.Sprintf("unable to add version '%s'. Adding a version with expiration date in the past is not allowed", version.Version)))
			continue
		}

		// added preview version must be latest patch version (implicitly higher than other preview and supported versions)
		if version.Classification != nil && *version.Classification == core.ClassificationPreview {
			latestVersion, err := helper.DetermineLatestExpirableVersion(versions)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(index), latestVersion, "failed to determine latest expirable version from cloud profile"))
			}
			if latestVersion.Version != version.Version {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(index), version.Version, fmt.Sprintf("unable to add version '%s'. Added '%s' versions have to be the latest patch version of a minor version.", version.Version, core.ClassificationPreview)))
			}
		}

		if version.Classification != nil && *version.Classification == core.ClassificationSupported {
			currentSemVer, err := semver.NewVersion(version.Version)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(index), version.Version, fmt.Sprintf("unable to add version '%s'. Version should be semVer compatible", version.Version)))
				continue
			}

			// do not allow adding a higher supported version than the preview version of the same minor
			filteredVersions, err := helper.FilterVersionsWithSameMajorMinor(*currentSemVer, helper.FilterVersionsWithClassification(versions, core.ClassificationPreview))
			if err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(index), currentSemVer.String(), fmt.Sprintf("unable to add version '%s' : '%v'", version.Version, err)))
				continue
			}

			isLower, err := helper.VersionIsLowerOrEqualThan(*currentSemVer, filteredVersions)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(index), currentSemVer.String(), fmt.Sprintf("unable to add version '%s'. Could not determine if '%s' version is lower than '%s' version. This should not happen: '%v'", version.Version, core.ClassificationSupported, core.ClassificationPreview, err)))
			}
			if !isLower {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(index), currentSemVer.String(), fmt.Sprintf("unable to add version '%s'. Version with classification '%s' cannot be higher than any preview version of that minor. First, promote preview version to supported and then add this version as a preview version.", version.Version, core.ClassificationSupported)))
			}
		}
	}
	return allErrs
}

// validateCloudProfileVersionsUpdate validates versions both removed and added to the CloudProfile
func validateCloudProfileVersionsUpdate(new, old []core.ExpirableVersion, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateCloudProfileAddedVersions(new, helper.GetAddedVersions(old, new), fldPath)...)

	return allErrs
}

// ValidateCloudProfileSpec validates the specification of a CloudProfile object.
func ValidateCloudProfileSpec(spec *core.CloudProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must provide a provider type"))
	}

	allErrs = append(allErrs, validateKubernetesSettings(spec.Kubernetes, fldPath.Child("kubernetes"))...)
	allErrs = append(allErrs, validateMachineImages(spec.MachineImages, fldPath.Child("machineImages"))...)
	allErrs = append(allErrs, validateMachineTypes(spec.MachineTypes, fldPath.Child("machineTypes"))...)
	allErrs = append(allErrs, validateVolumeTypes(spec.VolumeTypes, fldPath.Child("volumeTypes"))...)
	allErrs = append(allErrs, validateRegions(spec.Regions, fldPath.Child("regions"))...)
	if spec.SeedSelector != nil {
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(spec.SeedSelector, fldPath.Child("seedSelector"))...)
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
	latestKubernetesVersion, err := helper.DetermineLatestExpirableVersion(kubernetes.Versions)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, latestKubernetesVersion, "failed to determine latest kubernetes version from cloud profile"))
	}
	if latestKubernetesVersion.ExpirationDate != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("versions[]").Child("expirationDate"), latestKubernetesVersion.ExpirationDate, fmt.Sprintf("expiration date of latest kubernetes version ('%s') must not be set", latestKubernetesVersion.Version)))
	}

	versionsFound := sets.NewString()
	r, _ := regexp.Compile(`^([0-9]+\.){2}[0-9]+$`)
	for i, version := range kubernetes.Versions {
		idxPath := fldPath.Child("versions").Index(i)
		if !r.MatchString(version.Version) {
			allErrs = append(allErrs, field.Invalid(idxPath, version, fmt.Sprintf("all Kubernetes versions must match the regex %s", r)))
		} else if versionsFound.Has(version.Version) {
			allErrs = append(allErrs, field.Duplicate(idxPath.Child("version"), version.Version))
		} else {
			versionsFound.Insert(version.Version)
		}
		allErrs = append(allErrs, validateExpirableVersion(version, idxPath)...)
	}

	return allErrs
}

func validateExpirableVersion(version core.ExpirableVersion, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if version.Classification != nil && !core.SupportedVersionClassifications.Has(string(*version.Classification)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("classification"), *version.Classification, core.SupportedVersionClassifications.List()))
	}

	return allErrs
}

func validateMachineTypes(machineTypes []core.MachineType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineTypes) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine type"))
	}

	names := make(map[string]struct{}, len(machineTypes))

	for i, machineType := range machineTypes {
		idxPath := fldPath.Index(i)
		namePath := idxPath.Child("name")
		cpuPath := idxPath.Child("cpu")
		gpuPath := idxPath.Child("gpu")
		memoryPath := idxPath.Child("memory")

		if len(machineType.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a name"))
		}

		if _, ok := names[machineType.Name]; ok {
			allErrs = append(allErrs, field.Duplicate(namePath, machineType.Name))
			break
		}
		names[machineType.Name] = struct{}{}

		allErrs = append(allErrs, validateResourceQuantityValue("cpu", machineType.CPU, cpuPath)...)
		allErrs = append(allErrs, validateResourceQuantityValue("gpu", machineType.GPU, gpuPath)...)
		allErrs = append(allErrs, validateResourceQuantityValue("memory", machineType.Memory, memoryPath)...)
	}

	return allErrs
}

func validateMachineImages(machineImages []core.MachineImage, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(machineImages) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine image"))
	}

	latestMachineImages, err := helper.DetermineLatestMachineImageVersions(machineImages)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, latestMachineImages, "failed to determine latest machine images from cloud profile"))
	}

	for imageName, latestImage := range latestMachineImages {
		if latestImage.ExpirationDate != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("expirationDate"), latestImage.ExpirationDate, fmt.Sprintf("expiration date of latest image ('%s','%s') must not be set", imageName, latestImage.Version)))
		}
	}

	duplicateNameVersion := sets.String{}
	duplicateName := sets.String{}
	for i, image := range machineImages {
		idxPath := fldPath.Index(i)
		if duplicateName.Has(image.Name) {
			allErrs = append(allErrs, field.Duplicate(idxPath, image.Name))
		}
		duplicateName.Insert(image.Name)

		if len(image.Name) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("name"), "machine image name must not be empty"))
		}

		if len(image.Versions) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("versions"), fmt.Sprintf("must provide at least one version for the machine image '%s'", image.Name)))
		}

		for index, machineVersion := range image.Versions {
			versionsPath := idxPath.Child("versions").Index(index)
			key := fmt.Sprintf("%s-%s", image.Name, machineVersion.Version)
			if duplicateNameVersion.Has(key) {
				allErrs = append(allErrs, field.Duplicate(versionsPath, key))
			}
			duplicateNameVersion.Insert(key)
			if len(machineVersion.Version) == 0 {
				allErrs = append(allErrs, field.Required(versionsPath.Child("version"), machineVersion.Version))
			}

			_, err := semver.NewVersion(machineVersion.Version)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(versionsPath.Child("version"), machineVersion.Version, "could not parse version. Use SemanticVersioning. In case there is no semVer version for this image use the extensibility provider (define mapping in the CloudProfile) to map to the actual non-semVer version"))
			}
			allErrs = append(allErrs, validateExpirableVersion(machineVersion, versionsPath)...)
		}
	}

	return allErrs
}

func validateVolumeTypes(volumeTypes []core.VolumeType, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	names := make(map[string]struct{}, len(volumeTypes))

	for i, volumeType := range volumeTypes {
		idxPath := fldPath.Index(i)
		namePath := idxPath.Child("name")
		classPath := idxPath.Child("class")

		if len(volumeType.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a name"))
		}

		if _, ok := names[volumeType.Name]; ok {
			allErrs = append(allErrs, field.Duplicate(namePath, volumeType.Name))
			break
		}
		names[volumeType.Name] = struct{}{}

		if len(volumeType.Class) == 0 {
			allErrs = append(allErrs, field.Required(classPath, "must provide a class"))
		}
	}

	return allErrs
}

func validateRegions(regions []core.Region, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(regions) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one region"))
	}

	regionsFound := sets.NewString()
	for i, region := range regions {
		idxPath := fldPath.Index(i)
		namePath := idxPath.Child("name")
		zonesPath := idxPath.Child("zones")

		if len(region.Name) == 0 {
			allErrs = append(allErrs, field.Required(namePath, "must provide a region name"))
		} else if regionsFound.Has(region.Name) {
			allErrs = append(allErrs, field.Duplicate(namePath, region.Name))
		} else {
			regionsFound.Insert(region.Name)
		}

		zonesFound := sets.NewString()
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
	}
	return allErrs
}
