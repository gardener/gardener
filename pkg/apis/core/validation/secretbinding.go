// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
)

// ValidateSecretBinding validates a SecretBinding object.
func ValidateSecretBinding(binding *core.SecretBinding) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&binding.ObjectMeta, true, ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateSecretReferenceOptionalNamespace(binding.SecretRef, field.NewPath("secretRef"))...)
	for i, quota := range binding.Quotas {
		allErrs = append(allErrs, ValidateObjectReferenceNameAndNamespace(quota, field.NewPath("quotas").Index(i), false)...)
	}

	return allErrs
}

// ValidateSecretBindingUpdate validates a SecretBinding object before an update.
func ValidateSecretBindingUpdate(newBinding, oldBinding *core.SecretBinding) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newBinding.ObjectMeta, &oldBinding.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newBinding.SecretRef, oldBinding.SecretRef, field.NewPath("secretRef"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newBinding.Quotas, oldBinding.Quotas, field.NewPath("quotas"))...)
	if oldBinding.Provider != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newBinding.Provider, oldBinding.Provider, field.NewPath("provider"))...)
	}
	allErrs = append(allErrs, ValidateSecretBinding(newBinding)...)

	return allErrs
}

// ValidateSecretBindingProvider validates a SecretBindingProvider object.
func ValidateSecretBindingProvider(provider *core.SecretBindingProvider) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		fldPath = field.NewPath("provider")
	)

	if provider == nil {
		allErrs = append(allErrs, field.Required(fldPath, "must specify a provider"))
		return allErrs
	}

	if len(provider.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must specify a provider type"))
	}

	return allErrs
}

// ValidateAuditPolicyConfigMapReference validates the audit policy config map reference.
func ValidateAuditPolicyConfigMapReference(ref *corev1.ObjectReference, fldPath *field.Path) field.ErrorList {
	return ValidateObjectReferenceNameAndNamespace(*ref, fldPath, false)
}

func validateSecretReferenceOptionalNamespace(ref corev1.SecretReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}

	return allErrs
}
