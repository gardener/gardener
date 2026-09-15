// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// ExtensionShootServiceAccountName returns the name of the garden ServiceAccount for an extension in a self-hosted shoot cluster.
func ExtensionShootServiceAccountName(shootName, controllerInstallationName string) string {
	return v1beta1constants.ExtensionShootServiceAccountPrefix + shootName + "--" + controllerInstallationName
}

// ParseExtensionShootServiceAccountName parses the shoot name out of an
// `extension-shoot--<shootName>--<controllerInstallationName>` ServiceAccount name. Returns false if
// the name does not carry the prefix or the expected two-separator structure.
func ParseExtensionShootServiceAccountName(serviceAccountName string) (string, bool) {
	withoutPrefix, ok := strings.CutPrefix(serviceAccountName, v1beta1constants.ExtensionShootServiceAccountPrefix)
	if !ok {
		return "", false
	}
	shootName, _, found := strings.Cut(withoutPrefix, "--")
	if !found || shootName == "" {
		return "", false
	}
	return shootName, true
}
