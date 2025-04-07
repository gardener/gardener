// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"
)

// ExtensionRuntimeManagedResourceName returns the name of the ManagedResource containing resources for the Garden runtime cluster.
func ExtensionRuntimeManagedResourceName(extensionName string) string {
	return fmt.Sprintf("extension-%s-garden", extensionName)
}

// ExtensionRuntimeNamespaceName returns the name of the namespace hosting resources for the Garden runtime cluster.
func ExtensionRuntimeNamespaceName(extensionName string) string {
	return fmt.Sprintf("runtime-extension-%s", extensionName)
}
