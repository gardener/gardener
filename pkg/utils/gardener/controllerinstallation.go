// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ManagedResourceNameForControllerInstallation returns the name of the ManagedResource for the given
// ControllerInstallation. For self-hosted shoot ControllerInstallations, this is the registration name (matching the
// ManagedResource created during bootstrapping). For seed ControllerInstallations, it is the ControllerInstallation
// name.
func ManagedResourceNameForControllerInstallation(controllerInstallation *gardencorev1beta1.ControllerInstallation) string {
	if controllerInstallation.Spec.ShootRef != nil {
		return controllerInstallation.Spec.RegistrationRef.Name
	}
	return controllerInstallation.Name
}

// NamespaceNameForControllerInstallation returns the name of the namespace that will be used for the extension
// controller in the seed or self-hosted shoot cluster.
func NamespaceNameForControllerInstallation(controllerInstallation *gardencorev1beta1.ControllerInstallation) string {
	return "extension-" + ManagedResourceNameForControllerInstallation(controllerInstallation)
}
