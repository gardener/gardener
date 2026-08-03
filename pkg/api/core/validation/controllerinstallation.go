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

	"github.com/gardener/gardener/pkg/apis/core"
)

// ValidateControllerInstallation validates a ControllerInstallation object.
func ValidateControllerInstallation(controllerInstallation *core.ControllerInstallation) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&controllerInstallation.ObjectMeta, false, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateControllerInstallationSpec(&controllerInstallation.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateControllerInstallationUpdate validates a ControllerInstallation object before an update.
func ValidateControllerInstallationUpdate(newControllerInstallation, oldControllerInstallation *core.ControllerInstallation) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newControllerInstallation.ObjectMeta, &oldControllerInstallation.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateControllerInstallationSpecUpdate(&newControllerInstallation.Spec, &oldControllerInstallation.Spec, newControllerInstallation.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateControllerInstallation(newControllerInstallation)...)

	return allErrs
}

// ValidateControllerInstallationSpec validates the specification of a ControllerInstallation object.
func ValidateControllerInstallationSpec(spec *core.ControllerInstallationSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	registrationRefPath := fldPath.Child("registrationRef")
	if len(spec.RegistrationRef.Name) == 0 {
		allErrs = append(allErrs, field.Required(registrationRefPath.Child("name"), "field is required"))
	}

	if spec.SeedRef == nil && spec.ShootRef == nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("seedRef"), "either seedRef or shootRef must be set"))
	}

	if spec.SeedRef != nil {
		if len(spec.SeedRef.Name) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("seedRef").Child("name"), "field is required"))
		}
	}

	if spec.ShootRef != nil {
		if len(spec.ShootRef.Name) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("shootRef").Child("name"), "field is required"))
		}
		if len(spec.ShootRef.Namespace) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("shootRef").Child("namespace"), "field is required"))
		}
	}

	return allErrs
}

// ValidateControllerInstallationSpecUpdate validates the spec of a ControllerInstallation object before an update.
func ValidateControllerInstallationSpecUpdate(newSpec, oldSpec *core.ControllerInstallationSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(newSpec, oldSpec) {
		diff := deep.Equal(newSpec, oldSpec)
		return field.ErrorList{field.Forbidden(fldPath, fmt.Sprintf("cannot update controller installation spec if deletion timestamp is set. Requested changes: %s", strings.Join(diff, ",")))}
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.RegistrationRef.Name, oldSpec.RegistrationRef.Name, fldPath.Child("registrationRef", "name"))...)

	if oldSpec.SeedRef != nil && newSpec.SeedRef != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.SeedRef.Name, oldSpec.SeedRef.Name, fldPath.Child("seedRef", "name"))...)
	}

	if oldSpec.ShootRef == nil || newSpec.ShootRef == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.ShootRef, oldSpec.ShootRef, fldPath.Child("shootRef"))...)
	} else if newSpec.ShootRef != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.ShootRef.Name, oldSpec.ShootRef.Name, fldPath.Child("shootRef", "name"))...)
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.ShootRef.Namespace, oldSpec.ShootRef.Namespace, fldPath.Child("shootRef", "namespace"))...)
	}

	return allErrs
}

// ValidateControllerInstallationStatusUpdate validates the status field of a ControllerInstallation object.
func ValidateControllerInstallationStatusUpdate(_, _ core.ControllerInstallationStatus) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
