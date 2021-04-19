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
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateShootExtensionStatus validates a ShootExtensionStatus object
func ValidateShootExtensionStatus(ShootExtensionStatus *core.ShootExtensionStatus) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&ShootExtensionStatus.ObjectMeta, true, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateShootExtensionStatuses(ShootExtensionStatus.Statuses, field.NewPath("statuses"))...)

	return allErrs
}

// ValidateShootExtensionStatusUpdate validates an update to a ShootExtensionStatus object
func ValidateShootExtensionStatusUpdate(newShootExtensionStatus, oldShootExtensionStatus *core.ShootExtensionStatus) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newShootExtensionStatus.ObjectMeta, &oldShootExtensionStatus.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateShootExtensionStatusExtensionsUpdate(newShootExtensionStatus.Statuses, oldShootExtensionStatus.Statuses)...)
	allErrs = append(allErrs, ValidateShootExtensionStatus(newShootExtensionStatus)...)

	return allErrs
}

// ValidateShootExtensionStatuses validates ExtensionStatuses.
func ValidateShootExtensionStatuses(statuses []core.ExtensionStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, data := range statuses {
		idxPath := fldPath.Index(i)

		if len(data.Kind) == 0 {
			kindPath := idxPath.Child("kind")
			allErrs = append(allErrs, field.Invalid(kindPath, data.Kind, "the kind field of the extension status cannot be empty"))
		}

		if len(data.Type) == 0 {
			typePath := idxPath.Child("type")
			allErrs = append(allErrs, field.Invalid(typePath, data.Kind, "the type field of the extension status cannot be empty"))
		}
	}

	return allErrs
}

// ValidateShootExtensionStatusExtensionsUpdate validates the update of ExtensionStatuses.
func ValidateShootExtensionStatusExtensionsUpdate(new, old []core.ExtensionStatus) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		fldPath = field.NewPath("statuses")
	)

	// this is an O(n square) operation.
	// But it does not matter as we only have a very small amount of extensions  (< 20)
	for i, dataOld := range old {
		idxPath := fldPath.Index(i)
		for _, dataNew := range new {
			if dataOld.Kind == dataNew.Kind && dataOld.Type != dataNew.Type {
				allErrs = append(allErrs, apivalidation.ValidateImmutableField(dataNew.Type, dataOld.Type, idxPath.Child("type"))...)
			}
		}
	}

	return allErrs
}
