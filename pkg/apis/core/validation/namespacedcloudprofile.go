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
func ValidateNamespacedCloudProfile(cloudProfile *core.NamespacedCloudProfile) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&cloudProfile.ObjectMeta, true, ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateNamespacedCloudProfileSpec(&cloudProfile.Spec, field.NewPath("spec"))...)

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

// validateNamespacedCloudProfileSpec validates the specification of a NamespacedCloudProfile object.
func validateNamespacedCloudProfileSpec(spec *core.NamespacedCloudProfileSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateParent(spec.Parent, fldPath.Child("parent"))...)

	if spec.Kubernetes != nil {
		allErrs = append(allErrs, validateKubernetesSettings(*spec.Kubernetes, fldPath.Child("kubernetes"))...)
	}
	if spec.MachineImages != nil {
		allErrs = append(allErrs, validateMachineImages(spec.MachineImages, fldPath.Child("machineImages"))...)
	}
	if spec.MachineTypes != nil {
		allErrs = append(allErrs, validateMachineTypes(spec.MachineTypes, fldPath.Child("machineTypes"))...)
	}
	if spec.VolumeTypes != nil {
		allErrs = append(allErrs, validateVolumeTypes(spec.VolumeTypes, fldPath.Child("volumeTypes"))...)
	}
	if spec.Regions != nil {
		allErrs = append(allErrs, validateRegions(spec.Regions, fldPath.Child("regions"))...)
	}
	if spec.CABundle != nil {
		_, err := utils.DecodeCertificate([]byte(*(spec.CABundle)))
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("caBundle"), *(spec.CABundle), "caBundle is not a valid PEM-encoded certificate"))
		}
	}

	return allErrs
}

func validateParent(parent core.CloudProfileReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if parent.Kind != "CloudProfile" {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("kind"), parent.Kind, []string{"CloudProfile"}))
	}
	if len(parent.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a parent name"))
	}

	return allErrs
}
