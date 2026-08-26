// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// SetDefaults_AdminKubeconfigRequestSpec sets default values for AdminKubeconfigRequestSpec objects.
func SetDefaults_AdminKubeconfigRequestSpec(obj *AdminKubeconfigRequestSpec) {
	if obj.ExpirationSeconds == nil {
		obj.ExpirationSeconds = new(int64(60 * 60))
	}
}
