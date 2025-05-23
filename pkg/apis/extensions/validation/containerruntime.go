// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
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
		if diff := deep.Equal(new, old); diff != nil {
			return field.ErrorList{field.Forbidden(fldPath, strings.Join(diff, ","))}
		}
		return apivalidation.ValidateImmutableField(new, old, fldPath)
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.WorkerPool.Name, old.WorkerPool.Name, fldPath.Child("workerPool", "name"))...)

	return allErrs
}
