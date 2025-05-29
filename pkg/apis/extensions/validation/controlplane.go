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

// ValidateControlPlane validates a ControlPlane object.
func ValidateControlPlane(cp *extensionsv1alpha1.ControlPlane) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&cp.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateControlPlaneSpec(&cp.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateControlPlaneUpdate validates a ControlPlane object before an update.
func ValidateControlPlaneUpdate(new, old *extensionsv1alpha1.ControlPlane) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateControlPlaneSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateControlPlane(new)...)

	return allErrs
}

// ValidateControlPlaneSpec validates the specification of a ControlPlane object.
func ValidateControlPlaneSpec(spec *extensionsv1alpha1.ControlPlaneSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field is required"))
	}

	if spec.Purpose != nil {
		if *spec.Purpose != extensionsv1alpha1.Normal && *spec.Purpose != extensionsv1alpha1.Exposure {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("purpose"), *spec.Purpose, []string{string(extensionsv1alpha1.Normal), string(extensionsv1alpha1.Exposure)}))
		}
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
func ValidateControlPlaneSpecUpdate(new, old *extensionsv1alpha1.ControlPlaneSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		diff := deep.Equal(new, old)
		return field.ErrorList{field.Forbidden(fldPath, fmt.Sprintf("cannot update control plane spec if deletion timestamp is set. Requested changes: %s", strings.Join(diff, ",")))}
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Purpose, old.Purpose, fldPath.Child("purpose"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Region, old.Region, fldPath.Child("region"))...)

	return allErrs
}
