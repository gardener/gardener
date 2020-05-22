// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateShootState validates a ShootState object
func ValidateShootState(shootState *core.ShootState) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&shootState.ObjectMeta, true, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateShootStateSpec(&shootState.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateShootStateUpdate validates an update to a ShootState object
func ValidateShootStateUpdate(newShootState, oldShootState *core.ShootState) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newShootState.ObjectMeta, &oldShootState.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateShootStateSpecUpdate(&newShootState.Spec, &oldShootState.Spec)...)
	allErrs = append(allErrs, ValidateShootState(newShootState)...)

	return allErrs
}

// ValidateShootStateSpec validates the spec field of a ShootState object.
func ValidateShootStateSpec(shootStateSpec *core.ShootStateSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, data := range shootStateSpec.Gardener {
		idxPath := fldPath.Child("gardener").Index(i)
		namePath := idxPath.Child("name")

		if len(data.Name) == 0 {
			allErrs = append(allErrs, field.Invalid(namePath, data.Name, "name of data required to generate resources cannot be empty"))
		}
	}

	for i, extension := range shootStateSpec.Extensions {
		idxPath := fldPath.Child("extensions").Index(i)
		kindPath := idxPath.Child("kind")
		purposePath := idxPath.Child("purpose")

		if len(extension.Kind) == 0 {
			allErrs = append(allErrs, field.Invalid(kindPath, extension.Kind, "extension resource kind cannot be empty"))
		}
		if extension.Purpose != nil && len(*extension.Purpose) == 0 {
			allErrs = append(allErrs, field.Invalid(purposePath, extension.Purpose, "extension resource purpose cannot be empty"))
		}
		allErrs = append(allErrs, validateResources(extension.Resources, fldPath.Child("resources"))...)
	}

	for i, resource := range shootStateSpec.Resources {
		allErrs = append(allErrs, validateCrossVersionObjectReference(resource.CrossVersionObjectReference, fldPath.Child("resources").Index(i))...)
	}

	return allErrs
}

// ValidateShootStateSpecUpdate validates the update to the specification of a ShootState
func ValidateShootStateSpecUpdate(newShootState, oldShootState *core.ShootStateSpec) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
