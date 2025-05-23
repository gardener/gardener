// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
)

var availableGardenletOperations = availableManagedSeedOperations.Clone().Insert(v1beta1constants.OperationForceRedeploy)

// ValidateGardenlet validates a Gardenlet object.
func ValidateGardenlet(gardenlet *seedmanagement.Gardenlet) field.ErrorList {
	var allErrs = field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&gardenlet.ObjectMeta, true, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	if gardenlet.Namespace != v1beta1constants.GardenNamespace {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("metadata", "namespace"), "namespace must be garden"))
	}
	allErrs = append(allErrs, validateOperation(gardenlet.Annotations[v1beta1constants.GardenerOperation], availableGardenletOperations, field.NewPath("metadata", "annotations").Key(v1beta1constants.GardenerOperation))...)
	allErrs = append(allErrs, ValidateGardenletSpec(&gardenlet.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateGardenletUpdate validates a Gardenlet object before an update.
func ValidateGardenletUpdate(newGardenlet, oldGardenlet *seedmanagement.Gardenlet) field.ErrorList {
	var allErrs = field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newGardenlet.ObjectMeta, &oldGardenlet.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateOperationUpdate(newGardenlet.Annotations[v1beta1constants.GardenerOperation], oldGardenlet.Annotations[v1beta1constants.GardenerOperation], field.NewPath("metadata", "annotations").Key(v1beta1constants.GardenerOperation))...)
	allErrs = append(allErrs, ValidateGardenletSpecUpdate(&newGardenlet.Spec, &oldGardenlet.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateGardenlet(newGardenlet)...)

	return allErrs
}

// ValidateGardenletSpec validates the specification of a Gardenlet object.
func ValidateGardenletSpec(spec *seedmanagement.GardenletSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateGardenletDeployment(&spec.Deployment.GardenletDeployment, fldPath.Child("deployment"))...)

	allErrs = append(allErrs, validation.ValidateOCIRepository(&spec.Deployment.Helm.OCIRepository, fldPath.Child("deployment", "helm", "ociRepository"))...)

	if spec.Config != nil {
		allErrs = append(allErrs, validateGardenletConfig(spec.Config, seedmanagement.BootstrapToken, false, fldPath.Child("config"), false)...)
	}

	return allErrs
}

// ValidateGardenletSpecUpdate validates the specification updates of a Gardenlet object.
func ValidateGardenletSpecUpdate(newSpec, oldSpec *seedmanagement.GardenletSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateGardenletConfigUpdate(newSpec.Config, oldSpec.Config, fldPath.Child("config"))...)

	return allErrs
}

// ValidateGardenletStatus validates the given GardenletStatus.
func ValidateGardenletStatus(status *seedmanagement.GardenletStatus, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure integer fields are non-negative
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(status.ObservedGeneration, fieldPath.Child("observedGeneration"))...)

	return allErrs
}

// ValidateGardenletStatusUpdate validates a Gardenlet object before a status update.
func ValidateGardenletStatusUpdate(newGardenlet, oldGardenlet *seedmanagement.Gardenlet) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newGardenlet.ObjectMeta, &oldGardenlet.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateGardenletStatus(&newGardenlet.Status, field.NewPath("status"))...)

	return allErrs
}
