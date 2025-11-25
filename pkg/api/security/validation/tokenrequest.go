// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"time"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/security"
)

// ValidateTokenRequest validates a TokenRequest.
func ValidateTokenRequest(request *security.TokenRequest) field.ErrorList {
	var (
		allErrs  = field.ErrorList{}
		specPath = field.NewPath("spec")
	)

	allErrs = append(allErrs, validateExpirationSeconds(request.Spec.ExpirationSeconds, specPath.Child("expirationSeconds"))...)

	if request.Spec.ContextObject != nil {
		allErrs = append(allErrs, validateContextObject(*request.Spec.ContextObject, specPath.Child("contextObject"))...)
	}

	return allErrs
}

func validateExpirationSeconds(expirationSeconds int64, path *field.Path) field.ErrorList {
	const (
		minDuration = time.Minute * 10
		maxDuration = 1 << 32
	)
	allErrs := field.ErrorList{}

	if expirationSeconds < int64(minDuration.Seconds()) {
		allErrs = append(allErrs, field.Invalid(path, expirationSeconds, "may not specify a duration shorter than 10 minutes"))
	}
	if expirationSeconds > maxDuration {
		allErrs = append(allErrs, field.Invalid(path, expirationSeconds, "may not specify a duration longer than 2^32 seconds"))
	}

	return allErrs
}

func validateContextObject(ctxObj security.ContextObject, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(ctxObj.APIVersion) == 0 {
		allErrs = append(allErrs, field.Required(path.Child("apiVersion"), "must provide an apiVersion"))
	}

	if len(ctxObj.Kind) == 0 {
		allErrs = append(allErrs, field.Required(path.Child("kind"), "must provide a kind"))
	}

	if len(ctxObj.Name) == 0 {
		allErrs = append(allErrs, field.Required(path.Child("name"), "must provide a name"))
	}

	for _, err := range validation.IsDNS1123Subdomain(ctxObj.Name) {
		allErrs = append(allErrs, field.Invalid(path.Child("name"), ctxObj.Name, err))
	}

	if ctxObj.Namespace != nil { // Namespace can be omitted when cluster scoped object is referenced.
		if len(*ctxObj.Namespace) == 0 {
			allErrs = append(allErrs, field.Required(path.Child("namespace"), "namespace name cannot be empty"))
		}
		for _, err := range validation.IsDNS1123Subdomain(*ctxObj.Namespace) {
			allErrs = append(allErrs, field.Invalid(path.Child("namespace"), *ctxObj.Namespace, err))
		}
	}

	return allErrs
}
