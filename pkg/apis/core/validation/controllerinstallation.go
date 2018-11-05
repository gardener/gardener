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

// ValidateControllerInstallation validates a ControllerInstallation object.
func ValidateControllerInstallation(controllerInstallation *core.ControllerInstallation) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&controllerInstallation.ObjectMeta, false, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateControllerInstallationSpec(&controllerInstallation.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateControllerInstallationUpdate validates a ControllerInstallation object before an update.
func ValidateControllerInstallationUpdate(new, old *core.ControllerInstallation) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateControllerInstallationSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateControllerInstallation(new)...)

	return allErrs
}

// ValidateControllerInstallationSpec validates the specification of a ControllerInstallation object.
func ValidateControllerInstallationSpec(spec *core.ControllerInstallationSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	registrationRefPath := fldPath.Child("registrationRef")
	if len(spec.RegistrationRef.Name) == 0 {
		allErrs = append(allErrs, field.Required(registrationRefPath.Child("name"), "field is required"))
	}

	seedRef := fldPath.Child("seedRef")
	if len(spec.SeedRef.Name) == 0 {
		allErrs = append(allErrs, field.Required(seedRef.Child("name"), "field is required"))
	}

	return allErrs
}

// ValidateControllerInstallationSpecUpdate validates the spec of a ControllerInstallation object before an update.
func ValidateControllerInstallationSpecUpdate(new, old *core.ControllerInstallationSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, fldPath)...)
		return allErrs
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.RegistrationRef.Name, old.RegistrationRef.Name, fldPath.Child("registrationRef", "name"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.SeedRef.Name, old.SeedRef.Name, fldPath.Child("seedRef", "name"))...)

	return allErrs
}

// ValidateControllerInstallationStatus validates the status of a ControllerInstallation object.
func ValidateControllerInstallationStatus(spec *core.ControllerInstallationStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

// ValidateControllerInstallationStatusUpdate validates the status field of a ControllerInstallation object.
func ValidateControllerInstallationStatusUpdate(newStatus, oldStatus core.ControllerInstallationStatus) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
