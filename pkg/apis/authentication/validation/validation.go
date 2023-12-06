/*
Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package validation contains methods to validate kinds in the
// authentication.k8s.io API group.
package validation

import (
	"math"
	"time"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/authentication"
)

// ValidateAdminKubeconfigRequest validates a AdminKubeconfigRequest.
func ValidateAdminKubeconfigRequest(acr *authentication.AdminKubeconfigRequest) field.ErrorList {
	allErrs := field.ErrorList{}
	specPath := field.NewPath("spec")

	const min = 10 * time.Minute
	if acr.Spec.ExpirationSeconds < int64(min.Seconds()) {
		allErrs = append(allErrs, field.Invalid(specPath.Child("expirationSeconds"), acr.Spec.ExpirationSeconds, "may not specify a duration less than 10 minutes"))
	}
	if acr.Spec.ExpirationSeconds > math.MaxUint32 {
		allErrs = append(allErrs, field.TooLong(specPath.Child("expirationSeconds"), acr.Spec.ExpirationSeconds, math.MaxUint32))
	}
	return allErrs
}
