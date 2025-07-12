// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package features

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

var (
	// vpaFeatureGateVersionRanges maps supported VPA feature gates with Kubernetes versions
	vpaFeatureGateVersionRanges = map[string]*FeatureGateVersionRange{
		"InPlaceOrRecreate": {VersionRange: versionutils.VersionRange{AddedInVersion: "1.33"}},
	}
)

// ValidateVpaFeatureGates validates the given Vertical Pod Autoscaler feature gates with the currently supported ones.
func ValidateVpaFeatureGates(featureGates map[string]bool, version string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for featureGate := range featureGates {
		versionRange, exists := vpaFeatureGateVersionRanges[featureGate]
		if !exists {
			allErrs = append(allErrs, field.Invalid(fldPath.Child(featureGate), featureGate, "unknown feature gate"))
			break
		}

		isSupported, err := versionRange.Contains(version)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child(featureGate), featureGate, err.Error()))
			break
		}

		if !isSupported {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child(featureGate), "not supported in Kubernetes version "+version))
		}
	}

	return allErrs
}
