// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

import (
	"github.com/gardener/gardener/pkg/apis/core"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateExposureClass validates a ExposureClass object.
func ValidateExposureClass(exposureClass *core.ExposureClass) field.ErrorList {
	var allErrs = field.ErrorList{}

	handlerNameLength := len(exposureClass.Handler)

	for _, errorMessage := range validation.IsDNS1123Label(exposureClass.Handler) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("handler"), exposureClass.Name, errorMessage))
	}

	// Restrict the max length of handler names to 41 characters to ensure that the exposureclass
	// handler default namespace scheme (istio-ingress-handler-{handler-name}, see GardenletConfiguration)
	// does not exceed the max amount of characters for namespaces.
	if handlerNameLength > 41 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("handler"), exposureClass.Name, "exposure class handler is restricted to 41 characters"))
	}

	if exposureClass.Scheduling != nil {
		if exposureClass.Scheduling.SeedSelector != nil {
			allErrs = append(allErrs, metav1validation.ValidateLabelSelector(&exposureClass.Scheduling.SeedSelector.LabelSelector, field.NewPath("scheduling", "seedSelector"))...)
		}
		allErrs = append(allErrs, ValidateTolerations(exposureClass.Scheduling.Tolerations, field.NewPath("scheduling", "tolerations"))...)
	}

	return allErrs
}

// ValidateExposureClassUpdate validates a ExposureClass object before an update.
func ValidateExposureClassUpdate(new, old *core.ExposureClass) field.ErrorList {
	var allErrs = field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(old.Handler, new.Handler, field.NewPath("handler"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(old.Scheduling, new.Scheduling, field.NewPath("scheduling"))...)
	return allErrs
}
