// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
)

// ValidateExposureClass validates a ExposureClass object.
func ValidateExposureClass(exposureClass *core.ExposureClass) field.ErrorList {
	var allErrs = field.ErrorList{}

	handlerNameLength := len(exposureClass.Handler)

	for _, errorMessage := range validation.IsDNS1123Label(exposureClass.Handler) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("handler"), exposureClass.Name, errorMessage))
	}

	// Restrict the max length of handler names to 34 characters to ensure that the exposureclass
	// handler default namespace scheme (istio-ingress-handler-{handler-name}--{zone}, see GardenletConfiguration)
	// does not exceed the max amount of characters for namespaces.
	if handlerNameLength > 34 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("handler"), exposureClass.Name, "exposure class handler is restricted to 34 characters"))
	}

	if exposureClass.Scheduling != nil {
		if exposureClass.Scheduling.SeedSelector != nil {
			allErrs = append(allErrs, metav1validation.ValidateLabelSelector(&exposureClass.Scheduling.SeedSelector.LabelSelector, metav1validation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: true}, field.NewPath("scheduling", "seedSelector"))...)
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
