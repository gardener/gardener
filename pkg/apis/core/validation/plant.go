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

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidatePlant validates a Plant object.
func ValidatePlant(plant *core.Plant) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&plant.ObjectMeta, true, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidatePlantSpec(&plant.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidatePlantUpdate validates a Plant object before an update.
func ValidatePlantUpdate(new, old *core.Plant) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidatePlantSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidatePlant(new)...)

	return allErrs
}

// ValidatePlantSpec validates the specification of a Plant object.
func ValidatePlantSpec(spec *core.PlantSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	registrationRefPath := fldPath.Child("secretRef")
	if len(spec.SecretRef.Name) == 0 {
		allErrs = append(allErrs, field.Required(registrationRefPath.Child("name"), "field is required"))
	}

	return allErrs
}

// ValidatePlantSpecUpdate validates the spec of a Plant object before an update.
func ValidatePlantSpecUpdate(new, old *core.PlantSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, fldPath)...)
		return allErrs
	}

	return allErrs
}

// ValidatePlantStatus validates the status of a Plant object.
func ValidatePlantStatus(spec *core.PlantStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

// ValidatePlantStatusUpdate validates the status field of a Plant object.
func ValidatePlantStatusUpdate(newStatus, oldStatus core.PlantStatus) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
