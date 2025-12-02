// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// HasContainerdConfiguration returns true if containerd is the configured CRI and has a proper configuration.
func HasContainerdConfiguration(criConfig *extensionsv1alpha1.CRIConfig) bool {
	return criConfig != nil && criConfig.Name == extensionsv1alpha1.CRINameContainerD && criConfig.Containerd != nil
}

// FilePathsFrom returns the paths for all the given files.
func FilePathsFrom(files []extensionsv1alpha1.File) []string {
	var out []string

	for _, file := range files {
		out = append(out, file.Path)
	}

	return out
}
