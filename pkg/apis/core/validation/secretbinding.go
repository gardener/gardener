// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"github.com/gardener/gardener/pkg/apis/core"

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateSecretBinding validates a SecretBinding object.
func ValidateSecretBinding(binding *core.SecretBinding) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&binding.ObjectMeta, true, ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateSecretReferenceOptionalNamespace(binding.SecretRef, field.NewPath("secretRef"))...)
	for i, quota := range binding.Quotas {
		allErrs = append(allErrs, validateObjectReferenceOptionalNamespace(quota, field.NewPath("quotas").Index(i))...)
	}

	return allErrs
}

// ValidateSecretBindingUpdate validates a SecretBinding object before an update.
func ValidateSecretBindingUpdate(newBinding, oldBinding *core.SecretBinding) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newBinding.ObjectMeta, &oldBinding.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newBinding.SecretRef, oldBinding.SecretRef, field.NewPath("secretRef"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newBinding.Quotas, oldBinding.Quotas, field.NewPath("quotas"))...)
	allErrs = append(allErrs, ValidateSecretBinding(newBinding)...)

	return allErrs
}

func validateAuditPolicyConfigMapReference(ref *corev1.ObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}

	return allErrs
}

func validateObjectReferenceOptionalNamespace(ref corev1.ObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}

	return allErrs
}

func validateSecretReferenceOptionalNamespace(ref corev1.SecretReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}

	return allErrs
}
