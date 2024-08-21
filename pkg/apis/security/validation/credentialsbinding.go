// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	gardencorevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/apis/security"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
)

// ValidateCredentialsBinding validates a CredentialsBinding.
func ValidateCredentialsBinding(binding *security.CredentialsBinding) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&binding.ObjectMeta, true, gardencorevalidation.ValidateName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateCredentials(binding.CredentialsRef, field.NewPath("credentialsRef"))...)
	allErrs = append(allErrs, ValidateCredentialsBindingProvider(binding.Provider, field.NewPath("provider"))...)
	for i, quota := range binding.Quotas {
		allErrs = append(allErrs, validateObjectReferenceOptionalNamespace(quota, field.NewPath("quotas").Index(i))...)
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

func validateObjectReferenceOptionalNamespace(ref corev1.ObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}

	return allErrs
}

func validateCredentials(ref corev1.ObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ref.APIVersion) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("apiVersion"), "must provide an apiVersion"))
	}

	if len(ref.Kind) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "must provide a kind"))
	}

	if len(ref.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "must provide a name"))
	}

	for _, err := range validation.IsDNS1123Subdomain(ref.Name) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), ref.Name, err))
	}

	if len(ref.Namespace) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), "must provide a namespace"))
	}

	for _, err := range validation.IsDNS1123Subdomain(ref.Namespace) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), ref.Namespace, err))
	}

	var (
		secret           = corev1.SchemeGroupVersion.WithKind("Secret")
		workloadIdentity = securityv1alpha1.SchemeGroupVersion.WithKind("WorkloadIdentity")

		allowedGVKs = sets.New(secret, workloadIdentity)
		validGVKs   = []string{secret.String(), workloadIdentity.String()}
	)

	if !allowedGVKs.Has(ref.GroupVersionKind()) {
		allErrs = append(allErrs, field.NotSupported(fldPath, ref.String(), validGVKs))
	}

	return allErrs
}
