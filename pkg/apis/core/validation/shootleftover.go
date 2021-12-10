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

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateShootLeftover validates a ShootLeftover object.
func ValidateShootLeftover(shootLeftover *core.ShootLeftover) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&shootLeftover.ObjectMeta, true, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateShootLeftoverSpec(&shootLeftover.Spec, field.NewPath("spec"), false)...)

	return allErrs
}

// ValidateShootLeftoverUpdate validates a ShootLeftover object before an update.
func ValidateShootLeftoverUpdate(newShootLeftover, oldShootLeftover *core.ShootLeftover) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newShootLeftover.ObjectMeta, &oldShootLeftover.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateShootLeftoverSpecUpdate(&newShootLeftover.Spec, &oldShootLeftover.Spec, newShootLeftover.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateShootLeftover(newShootLeftover)...)

	return allErrs
}

// ValidateShootLeftoverStatusUpdate validates a ShootLeftover object before a status update.
func ValidateShootLeftoverStatusUpdate(newShootLeftover, oldShootLeftover *core.ShootLeftover) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newShootLeftover.ObjectMeta, &oldShootLeftover.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateShootLeftoverStatus(&newShootLeftover.Status, field.NewPath("status"))...)

	return allErrs
}

// ValidateShootLeftoverSpec validates the specification of a ShootLeftover object.
func ValidateShootLeftoverSpec(spec *core.ShootLeftoverSpec, fldPath *field.Path, inTemplate bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if spec.SeedName == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("seedName"), "field is required"))
	}
	if spec.ShootName == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("shootName"), "field is required"))
	}
	if spec.TechnicalID == nil || *spec.TechnicalID == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("technicalID"), "field is required"))
	}
	if spec.UID == nil || *spec.UID == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("uid"), "field is required"))
	}

	return allErrs
}

// ValidateShootLeftoverSpecUpdate validates the specification updates of a ShootLeftover object.
func ValidateShootLeftoverSpecUpdate(newSpec, oldSpec *core.ShootLeftoverSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(newSpec, oldSpec) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec, oldSpec, fldPath)...)
		return allErrs
	}

	return allErrs
}

// ValidateShootLeftoverStatus validates the given ShootLeftoverStatus.
func ValidateShootLeftoverStatus(status *core.ShootLeftoverStatus, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(status.ObservedGeneration, fieldPath.Child("observedGeneration"))...)

	return allErrs
}
