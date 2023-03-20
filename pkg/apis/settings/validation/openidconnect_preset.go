// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
