// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// SetDefaults_ViewerKubeconfigRequestSpec sets default values for ViewerKubeconfigRequestSpec objects.
func SetDefaults_ViewerKubeconfigRequestSpec(obj *ViewerKubeconfigRequestSpec) {
	if obj.ExpirationSeconds == nil {
		obj.ExpirationSeconds = new(int64(60 * 60))
	}
}
