// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
)

// ValidateShootState validates a ShootState object
func ValidateShootState(shootState *core.ShootState) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&shootState.ObjectMeta, true, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateShootStateSpec(&shootState.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateShootStateUpdate validates an update to a ShootState object
func ValidateShootStateUpdate(newShootState, oldShootState *core.ShootState) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newShootState.ObjectMeta, &oldShootState.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateShootStateSpecUpdate(&newShootState.Spec, &oldShootState.Spec)...)
	allErrs = append(allErrs, ValidateShootState(newShootState)...)

	return allErrs
}

// ValidateShootStateSpec validates the spec field of a ShootState object.
func ValidateShootStateSpec(shootStateSpec *core.ShootStateSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, data := range shootStateSpec.Gardener {
		idxPath := fldPath.Child("gardener").Index(i)
		namePath := idxPath.Child("name")

		if len(data.Name) == 0 {
			allErrs = append(allErrs, field.Invalid(namePath, data.Name, "name of data required to generate resources cannot be empty"))
		}
	}

	for i, extension := range shootStateSpec.Extensions {
		idxPath := fldPath.Child("extensions").Index(i)
		kindPath := idxPath.Child("kind")
		purposePath := idxPath.Child("purpose")

		if len(extension.Kind) == 0 {
			allErrs = append(allErrs, field.Invalid(kindPath, extension.Kind, "extension resource kind cannot be empty"))
		}
		if extension.Purpose != nil && len(*extension.Purpose) == 0 {
			allErrs = append(allErrs, field.Invalid(purposePath, extension.Purpose, "extension resource purpose cannot be empty"))
		}
		allErrs = append(allErrs, validateResources(extension.Resources, fldPath.Child("resources"))...)
	}

	for i, resource := range shootStateSpec.Resources {
		allErrs = append(allErrs, validateCrossVersionObjectReference(resource.CrossVersionObjectReference, fldPath.Child("resources").Index(i))...)
	}

	return allErrs
}

// ValidateShootStateSpecUpdate validates the update to the specification of a ShootState
func ValidateShootStateSpecUpdate(_, _ *core.ShootStateSpec) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
