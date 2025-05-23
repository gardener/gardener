// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/settings"
)

// ValidateOpenIDConnectPreset validates a OpenIDConnectPreset object.
func ValidateOpenIDConnectPreset(oidc *settings.OpenIDConnectPreset) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&oidc.ObjectMeta, true, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateOpenIDConnectPresetSpec(&oidc.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateOpenIDConnectPresetUpdate validates a OpenIDConnectPreset object before an update.
func ValidateOpenIDConnectPresetUpdate(new, old *settings.OpenIDConnectPreset) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateOpenIDConnectPresetSpec(&new.Spec, field.NewPath("spec"))...)

	return allErrs
}
