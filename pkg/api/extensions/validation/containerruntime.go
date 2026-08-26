// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	"github.com/go-test/deep"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ValidateContainerRuntime validates a ContainerRuntime object.
func ValidateContainerRuntime(cr *extensionsv1alpha1.ContainerRuntime) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&cr.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateContainerRuntimeSpec(&cr.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateContainerRuntimeUpdate validates a ContainerRuntime object before an update.
func ValidateContainerRuntimeUpdate(newContainerRuntime, oldContainerRuntime *extensionsv1alpha1.ContainerRuntime) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newContainerRuntime.ObjectMeta, &oldContainerRuntime.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateContainerRuntimeSpecUpdate(&newContainerRuntime.Spec, &oldContainerRuntime.Spec, newContainerRuntime.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateContainerRuntime(newContainerRuntime)...)

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
func ValidateContainerRuntimeSpecUpdate(newSpec, oldSpec *extensionsv1alpha1.ContainerRuntimeSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(newSpec, oldSpec) {
		diff := deep.Equal(newSpec, oldSpec)
		return field.ErrorList{field.Forbidden(fldPath, fmt.Sprintf("cannot update container runtime spec if deletion timestamp is set. Requested changes: %s", strings.Join(diff, ",")))}
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Type, oldSpec.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.WorkerPool.Name, oldSpec.WorkerPool.Name, fldPath.Child("workerPool", "name"))...)

	return allErrs
}
