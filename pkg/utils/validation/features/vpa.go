// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package features

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	// vpaFeatureGates is a set of supported Vertical Pod Autoscaler feature gates
	vpaFeatureGates = sets.New(
		"InPlaceOrRecreate",
	)
)

// ValidateVpaFeatureGates validates the given Vertical Pod Autoscaler feature gates with the currently supported ones.
func ValidateVpaFeatureGates(featureGates map[string]bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for featureGate := range featureGates {
		if !vpaFeatureGates.Has(featureGate) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child(featureGate), featureGate, "unknown feature gate"))
			break
		}
	}
	return allErrs
}
