// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// NamespaceNameForControllerInstallation returns the name of the namespace that will be used for the extension controller in the seed.
func NamespaceNameForControllerInstallation(controllerInstallation *gardencorev1beta1.ControllerInstallation) string {
	return "extension-" + controllerInstallation.Name
}
