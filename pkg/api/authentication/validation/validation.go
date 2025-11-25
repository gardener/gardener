// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package validation contains methods to validate kinds in the
// authentication.k8s.io API group.
package validation

import (
	"math"
	"time"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/authentication"
)

// ValidateKubeconfigRequest validates a KubeconfigRequest.
func ValidateKubeconfigRequest(req *authentication.KubeconfigRequest) field.ErrorList {
	allErrs := field.ErrorList{}
	specPath := field.NewPath("spec")

	const min = 10 * time.Minute
	if req.Spec.ExpirationSeconds < int64(min.Seconds()) {
		allErrs = append(allErrs, field.Invalid(specPath.Child("expirationSeconds"), req.Spec.ExpirationSeconds, "may not specify a duration less than 10 minutes"))
	}
	if req.Spec.ExpirationSeconds > math.MaxUint32 {
		allErrs = append(allErrs, field.TooLong(specPath.Child("expirationSeconds"), req.Spec.ExpirationSeconds, math.MaxUint32))
	}
	return allErrs
}
