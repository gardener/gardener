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

// ValidateExtensionUpdate contains functionality for performing extended validation of an Extension object under update which
// is not possible with standard CRD validation, see https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules.
func ValidateExtensionUpdate(oldExtension, newExtension *operatorv1alpha1.Extension) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateControllerResourceUpdate(oldExtension.Spec.Resources, newExtension.Spec.Resources, field.NewPath("spec").Child("resources"))...)

	return allErrs
}

func validateControllerResourceUpdate(oldResources, newResources []gardencorev1beta1.ControllerResource, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var (
		oldCoreResources []gardencore.ControllerResource
		newCoreResources []gardencore.ControllerResource
	)

	for i, oldResource := range oldResources {
		oldCoreResource := &gardencore.ControllerResource{}
		if err := gardenCoreScheme.Convert(&oldResource, oldCoreResource, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(fldPath.Index(i), err))
		}

		// Imitate defaulting of ControllerRegistration
		if oldCoreResource.Primary == nil {
			oldCoreResource.Primary = ptr.To(true)
		}
		oldCoreResources = append(oldCoreResources, *oldCoreResource)
	}

	for i, newResource := range newResources {
		newCoreResource := &gardencore.ControllerResource{}
		if err := gardenCoreScheme.Convert(&newResource, newCoreResource, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(fldPath.Index(i), err))
		}

		// Imitate defaulting of ControllerRegistration
		if newCoreResource.Primary == nil {
			newCoreResource.Primary = ptr.To(true)
		}
		newCoreResources = append(newCoreResources, *newCoreResource)
	}

	allErrs = append(allErrs, gardencorevalidation.ValidateControllerResourceUpdate(newCoreResources, oldCoreResources, fldPath)...)

	return allErrs
}
