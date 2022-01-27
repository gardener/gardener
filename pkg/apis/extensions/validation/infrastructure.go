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
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateInfrastructure validates a Infrastructure object.
func ValidateInfrastructure(infra *extensionsv1alpha1.Infrastructure) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&infra.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateInfrastructureSpec(&infra.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateInfrastructureUpdate validates a Infrastructure object before an update.
func ValidateInfrastructureUpdate(new, old *extensionsv1alpha1.Infrastructure) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateInfrastructureSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateInfrastructure(new)...)

	return allErrs
}

// ValidateInfrastructureSpec validates the specification of a Infrastructure object.
func ValidateInfrastructureSpec(spec *extensionsv1alpha1.InfrastructureSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field is required"))
	}

	if len(spec.Region) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("region"), "field is required"))
	}

	if len(spec.SecretRef.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("secretRef", "name"), "field is required"))
	}

	return allErrs
}

// ValidateInfrastructureSpecUpdate validates the spec of a Infrastructure object before an update.
func ValidateInfrastructureSpecUpdate(new, old *extensionsv1alpha1.InfrastructureSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, fldPath)...)
		return allErrs
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Region, old.Region, fldPath.Child("region"))...)

	return allErrs
}

// ValidateInfrastructureStatus validates the status of a Infrastructure object.
func ValidateInfrastructureStatus(status *extensionsv1alpha1.InfrastructureStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

// ValidateInfrastructureStatusUpdate validates the status field of a Infrastructure object before an update.
func ValidateInfrastructureStatusUpdate(newStatus, oldStatus *extensionsv1alpha1.InfrastructureStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
