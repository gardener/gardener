// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

const (
	extensionAdmissionRuntimePrefix = "extension-admission-runtime-"
	extensionAdmissionVirtualPrefix = "extension-admission-virtual-"
	extensionRuntimePrefix          = "extension-"
	extensionRuntimeSuffix          = "-garden"
)

// ExtensionAdmissionRuntimeManagedResourceName returns the name of the ManagedResource containing resources for the Garden runtime cluster.
func ExtensionAdmissionRuntimeManagedResourceName(extensionName string) string {
	return extensionAdmissionRuntimePrefix + extensionName
}

// ExtensionAdmissionVirtualManagedResourceName returns the name of the ManagedResource containing resources for the Garden virtual cluster.
func ExtensionAdmissionVirtualManagedResourceName(extensionName string) string {
	return extensionAdmissionVirtualPrefix + extensionName
}

// ExtensionRuntimeManagedResourceName returns the name of the ManagedResource containing resources for the Garden runtime cluster.
func ExtensionRuntimeManagedResourceName(extensionName string) string {
	return extensionRuntimePrefix + extensionName + extensionRuntimeSuffix
}

// ExtensionForManagedResourceName returns if the given managed resource name belongs to an extension in Garden runtime cluster. If so, it returns the extension name.
func ExtensionForManagedResourceName(managedResourceName string) (string, bool) {
	if strings.HasPrefix(managedResourceName, extensionRuntimePrefix) && strings.HasSuffix(managedResourceName, extensionRuntimeSuffix) {
		return strings.TrimSuffix(strings.TrimPrefix(managedResourceName, extensionRuntimePrefix), extensionRuntimeSuffix), true
	}

	if strings.HasPrefix(managedResourceName, extensionAdmissionRuntimePrefix) {
		return strings.TrimPrefix(managedResourceName, extensionAdmissionRuntimePrefix), true
	}

	if strings.HasPrefix(managedResourceName, extensionAdmissionVirtualPrefix) {
		return strings.TrimPrefix(managedResourceName, extensionAdmissionVirtualPrefix), true
	}

	return "", false
}

// ExtensionRuntimeNamespacePrefix is the prefix for the namespace hosting resources for the Garden runtime cluster.
const ExtensionRuntimeNamespacePrefix = "runtime-extension-"

// ExtensionRuntimeNamespaceName returns the name of the namespace hosting resources for the Garden runtime cluster.
func ExtensionRuntimeNamespaceName(extensionName string) string {
	return ExtensionRuntimeNamespacePrefix + extensionName
}

// IsControllerInstallationInVirtualRequired returns true if the extension requires a controller installation in the virtual cluster.
func IsControllerInstallationInVirtualRequired(extension *operatorv1alpha1.Extension) bool {
	requiredCondition := v1beta1helper.GetCondition(extension.Status.Conditions, operatorv1alpha1.ExtensionRequiredVirtual)
	return requiredCondition != nil && requiredCondition.Status == gardencorev1beta1.ConditionTrue
}

// IsExtensionInRuntimeRequired returns true if the extension requires a deployment in the runtime cluster.
func IsExtensionInRuntimeRequired(extension *operatorv1alpha1.Extension) bool {
	requiredCondition := v1beta1helper.GetCondition(extension.Status.Conditions, operatorv1alpha1.ExtensionRequiredRuntime)
	return requiredCondition != nil && requiredCondition.Status == gardencorev1beta1.ConditionTrue
}

// ControllerRegistrationForExtension returns the ControllerRegistration and ControllerDeployment objects for the given
// Extension.
func ControllerRegistrationForExtension(extension *operatorv1alpha1.Extension) (*gardencorev1beta1.ControllerRegistration, *gardencorev1.ControllerDeployment) {
	resources := make([]gardencorev1beta1.ControllerResource, 0, len(extension.Spec.Resources))
	for _, resource := range extension.Spec.Resources {
		resource.AutoEnable = slices.DeleteFunc(slices.Clone(resource.AutoEnable), func(clusterType gardencorev1beta1.ClusterType) bool {
			return clusterType == operatorv1alpha1.ClusterTypeGarden
		})
		resource.ClusterCompatibility = slices.DeleteFunc(slices.Clone(resource.ClusterCompatibility), func(clusterType gardencorev1beta1.ClusterType) bool {
			return clusterType == operatorv1alpha1.ClusterTypeGarden
		})
		resources = append(resources, resource)
	}

	var (
		controllerDeployment = &gardencorev1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: extension.Name,
			},
			Helm: &gardencorev1.HelmControllerDeployment{
				Values:        extension.Spec.Deployment.ExtensionDeployment.Values,
				OCIRepository: extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository,
			},
			InjectGardenKubeconfig: extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig,
		}

		controllerRegistration = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: extension.Name,
			},
			Spec: gardencorev1beta1.ControllerRegistrationSpec{
				Resources: resources,
				Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
					Policy:       extension.Spec.Deployment.ExtensionDeployment.Policy,
					SeedSelector: extension.Spec.Deployment.ExtensionDeployment.SeedSelector,
					DeploymentRefs: []gardencorev1beta1.DeploymentRef{
						{
							Name: controllerDeployment.Name,
						},
					},
				},
			},
		}
	)

	if v, ok := extension.Annotations[v1beta1constants.AnnotationPodSecurityEnforce]; ok {
		metav1.SetMetaDataAnnotation(&controllerRegistration.ObjectMeta, v1beta1constants.AnnotationPodSecurityEnforce, v)
	}

	return controllerRegistration, controllerDeployment
}
