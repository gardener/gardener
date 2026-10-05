// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	"github.com/go-test/deep"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ValidateBastion validates a Bastion object.
func ValidateBastion(bastion *extensionsv1alpha1.Bastion) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&bastion.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBastionSpec(&bastion.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateBastionUpdate validates a Bastion object before an update.
func ValidateBastionUpdate(newBastion, oldBastion *extensionsv1alpha1.Bastion) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newBastion.ObjectMeta, &oldBastion.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBastionSpecUpdate(&newBastion.Spec, &oldBastion.Spec, newBastion.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateBastion(newBastion)...)

	return allErrs
}

// ValidateBastionSpec validates the specification of a Bastion object.
func ValidateBastionSpec(spec *extensionsv1alpha1.BastionSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field is required"))
	}

	if len(spec.UserData) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("userData"), "field is required"))
	}

	if len(spec.Ingress) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("ingress"), "field is required"))
	}

	if spec.Machine != nil {
		if spec.Machine.Type == nil && spec.Machine.Image == nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("machine"), spec.Machine, "at least one of type or image must be specified"))
		}
		if spec.Machine.Type != nil && len(*spec.Machine.Type) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("machine", "type"), *spec.Machine.Type, "machine type must not be empty"))
		}
		if spec.Machine.Image != nil {
			if len(spec.Machine.Image.Name) == 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("machine", "image", "name"), spec.Machine.Image.Name, "machine image name must not be empty"))
			}
			if spec.Machine.Image.Version != nil && len(*spec.Machine.Image.Version) == 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("machine", "image", "version"), *spec.Machine.Image.Version, "machine image version must not be empty"))
			}
		}
	}

	return allErrs
}

// ValidateBastionSpecUpdate validates the spec of a Bastion object before an update.
func ValidateBastionSpecUpdate(newSpec, oldSpec *extensionsv1alpha1.BastionSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(newSpec, oldSpec) {
		diff := deep.Equal(newSpec, oldSpec)
		return field.ErrorList{field.Forbidden(fldPath, fmt.Sprintf("cannot update bastion spec if deletion timestamp is set. Requested changes: %s", strings.Join(diff, ",")))}
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Type, oldSpec.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.UserData, oldSpec.UserData, fldPath.Child("userData"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Machine, oldSpec.Machine, fldPath.Child("machine"))...)

	return allErrs
}
