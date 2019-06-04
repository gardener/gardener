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
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateControllerRegistration validates a ControllerRegistration object.
func ValidateControllerRegistration(controllerRegistration *core.ControllerRegistration) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&controllerRegistration.ObjectMeta, false, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateControllerRegistrationSpec(&controllerRegistration.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateControllerRegistrationSpec validates the specification of a ControllerRegistration object.
func ValidateControllerRegistrationSpec(spec *core.ControllerRegistrationSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var (
		resources     = make(map[string]string, len(spec.Resources))
		resourcesPath = fldPath.Child("resources")
	)

	for i, resource := range spec.Resources {
		idxPath := resourcesPath.Index(i)

		if len(resource.Kind) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("kind"), "field is required"))
		}
		if len(resource.Type) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("type"), "field is required"))
		}
		if t, ok := resources[resource.Kind]; ok && t == resource.Type {
			allErrs = append(allErrs, field.Duplicate(idxPath, fmt.Sprintf("%s/%s", resource.Kind, resource.Type)))
		}
		if resource.GloballyEnabled != nil && resource.Kind != v1alpha1.ExtensionResource {
			allErrs = append(allErrs, field.Forbidden(idxPath.Child("globallyEnabled"), fmt.Sprintf("field must not be set when kind != %s", v1alpha1.ExtensionResource)))
		}

		resources[resource.Kind] = resource.Type
	}

	return allErrs
}

// ValidateControllerRegistrationUpdate validates a ControllerRegistration object before an update.
func ValidateControllerRegistrationUpdate(new, old *core.ControllerRegistration) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateControllerRegistrationSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateControllerRegistration(new)...)

	return allErrs
}

func ValidateControllerRegistrationSpecUpdate(new, old *core.ControllerRegistrationSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, fldPath)...)
		return allErrs
	}

	return allErrs
}
