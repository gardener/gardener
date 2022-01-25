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

// ValidateContainerRuntime validates a ContainerRuntime object.
func ValidateContainerRuntime(cr *extensionsv1alpha1.ContainerRuntime) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&cr.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateContainerRuntimeSpec(&cr.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateContainerRuntimeUpdate validates a ContainerRuntime object before an update.
func ValidateContainerRuntimeUpdate(new, old *extensionsv1alpha1.ContainerRuntime) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateContainerRuntimeSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateContainerRuntime(new)...)

	return allErrs
}

// ValidateContainerRuntimeSpec validates the spec of a ContainerRuntime object.
func ValidateContainerRuntimeSpec(spec *extensionsv1alpha1.ContainerRuntimeSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field is required"))
	}

	if len(spec.BinaryPath) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("binaryPath"), "field is required"))
	}

	if len(spec.WorkerPool.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("workerPool", "name"), "field is required"))
	}

	return allErrs
}

// ValidateContainerRuntimeSpecUpdate validates the spec of a ContainerRuntime object before an update.
func ValidateContainerRuntimeSpecUpdate(new, old *extensionsv1alpha1.ContainerRuntimeSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, fldPath)...)
		return allErrs
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.WorkerPool.Name, old.WorkerPool.Name, fldPath.Child("workerPool", "name"))...)

	return allErrs
}

// ValidateContainerRuntimeStatus validates the status of a ContainerRuntime object.
func ValidateContainerRuntimeStatus(status *extensionsv1alpha1.ContainerRuntimeStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

// ValidateContainerRuntimeStatusUpdate validates the status of a ContainerRuntime object before an update.
func ValidateContainerRuntimeStatusUpdate(newStatus, oldStatus *extensionsv1alpha1.ContainerRuntimeStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
