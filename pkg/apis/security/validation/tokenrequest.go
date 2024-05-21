// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	allErrs := field.ErrorList{}
	var (
		specPath = field.NewPath("spec")
	)
	const (
		minDuration = time.Minute * 10 // TODO(vpnachev): Make min duration configurable.
		maxDuration = time.Hour * 48   // TODO(vpnachev): Make max duration configurable.
	)
	if request.Spec.Duration != nil {
		if request.Spec.Duration.Duration < minDuration {
			allErrs = append(allErrs, field.Invalid(specPath.Child("duration"), request.Spec.Duration.String(), "may not specify a duration shorter than "+minDuration.String()))
		}
		if request.Spec.Duration.Duration > maxDuration {
			allErrs = append(allErrs, field.Invalid(specPath.Child("duration"), request.Spec.Duration.String(), "may not specify a duration longer than "+maxDuration.String()))
		}
	}

	if request.Spec.ContextObject != nil {
		allErrs = append(allErrs, validateContextObject(*request.Spec.ContextObject, specPath.Child("contextObject"))...)
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

	if len(ctxObj.Namespace) != 0 { // Namespace can be omitted when cluster scoped object is referenced.
		for _, err := range validation.IsDNS1123Subdomain(ctxObj.Namespace) {
			allErrs = append(allErrs, field.Invalid(path.Child("namespace"), ctxObj.Namespace, err))
		}
	}

	return allErrs
}
