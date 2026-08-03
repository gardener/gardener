// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

// ValidateExtension validates a Extension object.
func ValidateExtension(ext *extensionsv1alpha1.Extension) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&ext.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateExtensionSpec(&ext.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateExtensionUpdate validates a Extension object before an update.
func ValidateExtensionUpdate(newExtension, oldExtension *extensionsv1alpha1.Extension) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newExtension.ObjectMeta, &oldExtension.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateExtensionSpecUpdate(&newExtension.Spec, &oldExtension.Spec, newExtension.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateExtension(newExtension)...)

	return allErrs
}

// ValidateExtensionSpec validates the specification of a Extension object.
func ValidateExtensionSpec(spec *extensionsv1alpha1.ExtensionSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field is required"))
	}

	return allErrs
}

// ValidateExtensionSpecUpdate validates the spec of a Extension object before an update.
func ValidateExtensionSpecUpdate(newSpec, oldSpec *extensionsv1alpha1.ExtensionSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(newSpec, oldSpec) {
		diff := deep.Equal(newSpec, oldSpec)
		return field.ErrorList{field.Forbidden(fldPath, fmt.Sprintf("cannot update extension spec if deletion timestamp is set. Requested changes: %s", strings.Join(diff, ",")))}
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Type, oldSpec.Type, fldPath.Child("type"))...)

	return allErrs
}
