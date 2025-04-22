// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// ValidateExtension contains functionality for performing extended validation of an Extension object which
// is not possible with standard CRD validation, see https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules.
func ValidateExtension(extension *operatorv1alpha1.Extension) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateControllerResources(extension.Spec.Resources, field.NewPath("spec").Child("resources"))...)

	return allErrs
}

func validateControllerResources(resources []gardencorev1beta1.ControllerResource, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	coreResources, err := convertToCoreResources(resources)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
		return allErrs
	}

	validAutoEnablesModes := []gardencore.AutoEnableMode{
		gardencore.AutoEnableMode(operatorv1alpha1.AutoEnableModeGarden),
		gardencore.AutoEnableMode(gardencorev1beta1.AutoEnableModeSeed),
		gardencore.AutoEnableMode(gardencorev1beta1.AutoEnableModeShoot),
	}

	allErrs = append(allErrs, gardencorevalidation.ValidateControllerResources(coreResources, validAutoEnablesModes, fldPath)...)

	return allErrs
}

// ValidateExtensionUpdate contains functionality for performing extended validation of an Extension object under update which
// is not possible with standard CRD validation, see https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules.
func ValidateExtensionUpdate(oldExtension, newExtension *operatorv1alpha1.Extension) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateControllerResourcesUpdate(oldExtension.Spec.Resources, newExtension.Spec.Resources, field.NewPath("spec").Child("resources"))...)

	return allErrs
}

func validateControllerResourcesUpdate(oldResources, newResources []gardencorev1beta1.ControllerResource, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	oldCoreResources, err := convertToCoreResources(oldResources)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
		return allErrs
	}

	newCoreResources, err := convertToCoreResources(newResources)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
		return allErrs
	}

	allErrs = append(allErrs, gardencorevalidation.ValidateControllerResourcesUpdate(newCoreResources, oldCoreResources, fldPath)...)

	return allErrs
}

func convertToCoreResources(resources []gardencorev1beta1.ControllerResource) ([]gardencore.ControllerResource, error) {
	coreResources := make([]gardencore.ControllerResource, 0, len(resources))

	for _, oldResource := range resources {
		oldCoreResource := &gardencore.ControllerResource{}
		if err := gardenCoreScheme.Convert(&oldResource, oldCoreResource, nil); err != nil {
			return nil, err
		}

		// Imitate defaulting of ControllerRegistration, since defaulting for extensions was only added later.
		if oldCoreResource.Primary == nil {
			oldCoreResource.Primary = ptr.To(true)
		}
		coreResources = append(coreResources, *oldCoreResource)
	}

	return coreResources, nil
}
