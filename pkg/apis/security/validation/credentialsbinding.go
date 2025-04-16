// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"strings"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	gardencorevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/apis/security"
)

// ValidateCredentialsBinding validates a CredentialsBinding.
func ValidateCredentialsBinding(binding *security.CredentialsBinding) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&binding.ObjectMeta, true, gardencorevalidation.ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, gardencorevalidation.ValidateCredentialsRef(binding.CredentialsRef, field.NewPath("credentialsRef"))...)
	allErrs = append(allErrs, ValidateCredentialsBindingProvider(binding.Provider, field.NewPath("provider"))...)
	for i, quota := range binding.Quotas {
		allErrs = append(allErrs, gardencorevalidation.ValidateObjectReferenceNameAndNamespace(quota, field.NewPath("quotas").Index(i), false)...)
	}

	return allErrs
}

// ValidateCredentialsBindingUpdate validates a CredentialsBinding object before an update.
func ValidateCredentialsBindingUpdate(newBinding, oldBinding *security.CredentialsBinding) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newBinding.ObjectMeta, &oldBinding.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newBinding.CredentialsRef, oldBinding.CredentialsRef, field.NewPath("credentialsRef"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newBinding.Quotas, oldBinding.Quotas, field.NewPath("quotas"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newBinding.Provider, oldBinding.Provider, field.NewPath("provider"))...)

	allErrs = append(allErrs, ValidateCredentialsBinding(newBinding)...)

	return allErrs
}

// ValidateCredentialsBindingProvider validates a CredentialsBindingProvider object.
func ValidateCredentialsBindingProvider(provider security.CredentialsBindingProvider, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(provider.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must specify a provider type"))
	}

	if len(strings.Split(provider.Type, ",")) > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), provider.Type, "multiple providers specified"))
	}

	return allErrs
}
