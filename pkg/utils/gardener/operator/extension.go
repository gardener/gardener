// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"fmt"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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

// ExtensionRuntimeNamespaceName returns the name of the namespace hosting resources for the Garden runtime cluster.
func ExtensionRuntimeNamespaceName(extensionName string) string {
	return fmt.Sprintf("runtime-extension-%s", extensionName)
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
