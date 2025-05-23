// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/settings"
)

// ValidateClusterOpenIDConnectPreset validates a ClusterOpenIDConnectPreset object.
func ValidateClusterOpenIDConnectPreset(oidc *settings.ClusterOpenIDConnectPreset) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&oidc.ObjectMeta, false, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateClusterOpenIDConnectPresetSpec(&oidc.Spec, field.NewPath("spec"))...)
	return allErrs
}

// ValidateClusterOpenIDConnectPresetUpdate validates a ClusterOpenIDConnectPreset object before an update.
func ValidateClusterOpenIDConnectPresetUpdate(new, old *settings.ClusterOpenIDConnectPreset) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateClusterOpenIDConnectPresetSpec(&new.Spec, field.NewPath("spec"))...)

	return allErrs
}

func validateClusterOpenIDConnectPresetSpec(spec *settings.ClusterOpenIDConnectPresetSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, metav1validation.ValidateLabelSelector(spec.ProjectSelector, metav1validation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: true}, fldPath.Child("projectSelector"))...)
	allErrs = append(allErrs, validateOpenIDConnectPresetSpec(&spec.OpenIDConnectPresetSpec, fldPath)...)
	return allErrs
}
