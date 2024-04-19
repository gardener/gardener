// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/authentication"
	gardencorevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
)

// ValidateCredentialsBinding validates a CredentialsBinding.
func ValidateCredentialsBinding(binding *authentication.CredentialsBinding) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&binding.ObjectMeta, true, gardencorevalidation.ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateCredentialsRef(binding.CredentialsRef, field.NewPath("credentialsRef"))...)
	allErrs = append(allErrs, ValidateCredentialsBindingProvider(binding.Provider, field.NewPath("provider"))...)
	for i, quota := range binding.Quotas {
		allErrs = append(allErrs, validateObjectReferenceOptionalNamespace(quota, field.NewPath("quotas").Index(i))...)
	}

	return allErrs
}

// ValidateCredentialsBindingUpdate validates a CredentialsBinding object before an update.
func ValidateCredentialsBindingUpdate(newBinding, oldBinding *authentication.CredentialsBinding) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newBinding.ObjectMeta, &oldBinding.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newBinding.Quotas, oldBinding.Quotas, field.NewPath("quotas"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newBinding.Provider, oldBinding.Provider, field.NewPath("provider"))...)

	allErrs = append(allErrs, ValidateCredentialsBinding(newBinding)...)

	return allErrs
}

// ValidateCredentialsBindingProvider validates a CredentialsBindingProvider object.
func ValidateCredentialsBindingProvider(provider authentication.CredentialsBindingProvider, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(provider.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must specify a provider type"))
	}

	if len(strings.Split(provider.Type, ",")) > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), provider.Type, "multiple providers specified"))
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

func validateCredentialsRef(ref authentication.Credentials, fldPath *field.Path) field.ErrorList {
	if ref.Secret == nil && ref.WorkloadIdentity == nil {
		return field.ErrorList{field.Forbidden(fldPath, "must specify credentials provider")}
	}

	if ref.Secret != nil && ref.WorkloadIdentity != nil {
		return field.ErrorList{field.Forbidden(fldPath, "must specify exactly one credentials provider")}
	}

	if ref.Secret != nil {
		return validateSecret(ref.Secret, fldPath.Child("secret"))
	}
	return validateWorkloadIdentity(ref.WorkloadIdentity, fldPath.Child("workloadIdentity"))
}

func validateSecret(ref *corev1.SecretReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}
	return allErrs
}

func validateWorkloadIdentity(ref *authentication.WorkloadIdentityReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}
	return allErrs
}
