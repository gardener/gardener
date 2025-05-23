// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
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
func ValidateBastionUpdate(new, old *extensionsv1alpha1.Bastion) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBastionSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateBastion(new)...)

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

	return allErrs
}

// ValidateBastionSpecUpdate validates the spec of a Bastion object before an update.
func ValidateBastionSpecUpdate(new, old *extensionsv1alpha1.BastionSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		if diff := deep.Equal(new, old); diff != nil {
			return field.ErrorList{field.Forbidden(fldPath, strings.Join(diff, ","))}
		}
		return apivalidation.ValidateImmutableField(new, old, fldPath)
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.UserData, old.UserData, fldPath.Child("userData"))...)

	return allErrs
}
