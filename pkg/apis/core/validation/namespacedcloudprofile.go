// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/utils"
)

// ValidateNamespacedCloudProfile validates a NamespacedCloudProfile object.
func ValidateNamespacedCloudProfile(namespacedCloudProfile *core.NamespacedCloudProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&namespacedCloudProfile.ObjectMeta, true, ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateNamespacedCloudProfileParent(namespacedCloudProfile.Spec.Parent, field.NewPath("spec.parent"))...)

	allErrs = append(allErrs, validateNamespacedCloudProfileKubernetesVersions(namespacedCloudProfile.Spec.Kubernetes, field.NewPath("spec.kubernetes"))...)
	allErrs = append(allErrs, ValidateMachineImages(namespacedCloudProfile.Spec.MachineImages, field.NewPath("spec.machineImages"), true)...)
	allErrs = append(allErrs, validateVolumeTypes(namespacedCloudProfile.Spec.VolumeTypes, field.NewPath("spec.volumeTypes"))...)
	allErrs = append(allErrs, validateMachineTypes(namespacedCloudProfile.Spec.MachineTypes, field.NewPath("spec.machineTypes"))...)

	if namespacedCloudProfile.Spec.CABundle != nil {
		_, err := utils.DecodeCertificate([]byte(*(namespacedCloudProfile.Spec.CABundle)))
		if err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec.caBundle"), *(namespacedCloudProfile.Spec.CABundle), "caBundle is not a valid PEM-encoded certificate"))
		}
	}

	return allErrs
}

// ValidateNamespacedCloudProfileUpdate validates a NamespacedCloudProfile object before an update.
func ValidateNamespacedCloudProfileUpdate(newProfile, oldProfile *core.NamespacedCloudProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newProfile.ObjectMeta, &oldProfile.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateNamespacedCloudProfileSpecUpdate(&newProfile.Spec, &oldProfile.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateNamespacedCloudProfile(newProfile)...)

	return allErrs
}

// ValidateNamespacedCloudProfileSpecUpdate validates the spec update of a NamespacedCloudProfile.
func ValidateNamespacedCloudProfileSpecUpdate(oldProfile, newProfile *core.NamespacedCloudProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldProfile.Parent, newProfile.Parent, fldPath.Child("parent"))...)

	return allErrs
}

// ValidateNamespacedCloudProfileStatus validates the specification of a NamespacedCloudProfile object.
func ValidateNamespacedCloudProfileStatus(spec *core.CloudProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateCloudProfileKubernetesSettings(spec.Kubernetes, fldPath.Child("kubernetes"))...)
	if spec.MachineImages != nil {
		allErrs = append(allErrs, ValidateCloudProfileMachineImages(spec.MachineImages, fldPath.Child("machineImages"))...)
	}
	if spec.MachineTypes != nil {
		allErrs = append(allErrs, validateMachineTypes(spec.MachineTypes, fldPath.Child("machineTypes"))...)
	}
	if spec.VolumeTypes != nil {
		allErrs = append(allErrs, validateVolumeTypes(spec.VolumeTypes, fldPath.Child("volumeTypes"))...)
	}

	return allErrs
}

func validateNamespacedCloudProfileParent(parent core.CloudProfileReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if parent.Kind != "CloudProfile" {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("kind"), parent.Kind, []string{"CloudProfile"}))
	}
	if len(parent.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a parent name"))
	}

	return allErrs
}

func validateNamespacedCloudProfileKubernetesVersions(kubernetesSettings *core.KubernetesSettings, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if kubernetesSettings == nil {
		return allErrs
	}

	versions := kubernetesSettings.Versions
	for i, version := range versions {
		idxPath := fldPath.Child("versions").Index(i)
		if version.Classification != nil {
			allErrs = append(allErrs, field.Forbidden(idxPath.Child("classification"), "must not provide a classification to a Kubernetes version in NamespacedCloudProfile"))
		}
	}
	allErrs = append(allErrs, validateKubernetesVersions(versions, fldPath)...)
	return allErrs
}
