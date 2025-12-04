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

	allErrs = append(allErrs, validateExtensionSpec(extension.Spec, field.NewPath("spec"))...)

	return allErrs
}

func validateExtensionSpec(spec operatorv1alpha1.ExtensionSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateDeployment(spec.Deployment, fldPath.Child("deployment"))...)
	allErrs = append(allErrs, validateControllerResources(spec.Resources, fldPath.Child("resources"))...)

	return allErrs
}

func validateDeployment(deployment *operatorv1alpha1.Deployment, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deployment == nil {
		return append(allErrs, field.Required(fldPath, "deployment must be specified"))
	}
	if deployment.ExtensionDeployment == nil && deployment.AdmissionDeployment == nil {
		return append(allErrs, field.Required(fldPath, "at least one of extension or admission must be specified"))
	}

	allErrs = append(allErrs, validateExtensionDeployment(deployment.ExtensionDeployment, fldPath.Child("extension"))...)
	allErrs = append(allErrs, validateAdmissionDeployment(deployment.AdmissionDeployment, fldPath.Child("admission"))...)

	return allErrs
}

func validateExtensionDeployment(deployment *operatorv1alpha1.ExtensionDeploymentSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deployment == nil {
		return allErrs
	}

	allErrs = append(allErrs, validateHelmDeployment(deployment.Helm, fldPath.Child("helm"))...)

	return allErrs
}

func validateAdmissionDeployment(deployment *operatorv1alpha1.AdmissionDeploymentSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deployment == nil {
		return allErrs
	}

	if deployment.RuntimeCluster == nil && deployment.VirtualCluster == nil {
		return append(allErrs, field.Required(fldPath, "at least one of runtimeCluster or virtualCluster must be specified"))
	}

	if deployment.RuntimeCluster != nil {
		allErrs = append(allErrs, validateHelmDeployment(deployment.RuntimeCluster.Helm, fldPath.Child("runtimeCluster", "helm"))...)
	}

	if deployment.VirtualCluster != nil {
		allErrs = append(allErrs, validateHelmDeployment(deployment.VirtualCluster.Helm, fldPath.Child("virtualCluster", "helm"))...)
	}

	return allErrs
}

func validateHelmDeployment(helm *operatorv1alpha1.ExtensionHelm, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if helm == nil {
		return append(allErrs, field.Required(fldPath, "helm deployment must be specified"))
	}

	coreOCIRepo := &gardencore.OCIRepository{}
	if err := gardenCoreScheme.Convert(helm.OCIRepository, coreOCIRepo, nil); err != nil {
		return append(allErrs, field.InternalError(fldPath.Child("ociRepository"), err))
	}

	allErrs = append(allErrs, gardencorevalidation.ValidateOCIRepository(coreOCIRepo, fldPath.Child("ociRepository"))...)

	return allErrs
}

func validateControllerResources(resources []gardencorev1beta1.ControllerResource, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	coreResources, err := convertToCoreResources(resources)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
		return allErrs
	}

	validAutoEnabledModes := []gardencore.ClusterType{
		gardencore.ClusterType(operatorv1alpha1.ClusterTypeGarden),
		gardencore.ClusterType(gardencorev1beta1.ClusterTypeSeed),
		gardencore.ClusterType(gardencorev1beta1.ClusterTypeShoot),
	}

	allErrs = append(allErrs, gardencorevalidation.ValidateControllerResources(coreResources, validAutoEnabledModes, fldPath)...)

	return allErrs
}

// ValidateExtensionUpdate contains functionality for performing extended validation of an Extension object under update which
// is not possible with standard CRD validation, see https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules.
func ValidateExtensionUpdate(oldExtension, newExtension *operatorv1alpha1.Extension) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateExtensionSpec(newExtension.Spec, field.NewPath("spec"))...)
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
