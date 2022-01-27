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
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateBastion validates a Bastion object.
func ValidateBastion(bastion *extensionsv1alpha1.Bastion) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&bastion.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBastionSpec(&bastion.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateBastionUpdate validates a Bastion object before an update.
func ValidateBastionUpdate(new, old *extensionsv1alpha1.Bastion) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBastionSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateBastion(new)...)

	return allErrs
}

// ValidateBastionSpec validates the specification of a Bastion object.
func ValidateBastionSpec(spec *extensionsv1alpha1.BastionSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field is required"))
	}

	if len(spec.UserData) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("userData"), "field is required"))
	}

	if len(spec.Ingress) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("ingress"), "field is required"))
	}

	return allErrs
}

// ValidateBastionSpecUpdate validates the spec of a Bastion object before an update.
func ValidateBastionSpecUpdate(new, old *extensionsv1alpha1.BastionSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, fldPath)...)
		return allErrs
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.UserData, old.UserData, fldPath.Child("userData"))...)

	return allErrs
}

// ValidateBastionStatus validates the status of a Bastion object.
func ValidateBastionStatus(status *extensionsv1alpha1.BastionStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

// ValidateBastionStatusUpdate validates the status field of a Bastion object before an update.
func ValidateBastionStatusUpdate(newStatus, oldStatus *extensionsv1alpha1.BastionStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
