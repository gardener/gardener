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

// ValidateControlPlane validates a ControlPlane object.
func ValidateControlPlane(cp *extensionsv1alpha1.ControlPlane) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&cp.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateControlPlaneSpec(&cp.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateControlPlaneUpdate validates a ControlPlane object before an update.
func ValidateControlPlaneUpdate(newControlPlane, oldControlPlane *extensionsv1alpha1.ControlPlane) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newControlPlane.ObjectMeta, &oldControlPlane.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateControlPlaneSpecUpdate(&newControlPlane.Spec, &oldControlPlane.Spec, newControlPlane.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateControlPlane(newControlPlane)...)

	return allErrs
}

// ValidateControlPlaneSpec validates the specification of a ControlPlane object.
func ValidateControlPlaneSpec(spec *extensionsv1alpha1.ControlPlaneSpec, fldPath *field.Path) field.ErrorList {
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

// ValidateControlPlaneSpecUpdate validates the spec of a ControlPlane object before an update.
func ValidateControlPlaneSpecUpdate(newSpec, oldSpec *extensionsv1alpha1.ControlPlaneSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(newSpec, oldSpec) {
		diff := deep.Equal(newSpec, oldSpec)
		return field.ErrorList{field.Forbidden(fldPath, fmt.Sprintf("cannot update control plane spec if deletion timestamp is set. Requested changes: %s", strings.Join(diff, ",")))}
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Type, oldSpec.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Region, oldSpec.Region, fldPath.Child("region"))...)

	return allErrs
}
