// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/utils/ptr"
)

// SetDefaults_ViewerKubeconfigRequestSpec sets default values for ViewerKubeconfigRequestSpec objects.
func SetDefaults_ViewerKubeconfigRequestSpec(obj *ViewerKubeconfigRequestSpec) {
	if obj.ExpirationSeconds == nil {
		obj.ExpirationSeconds = ptr.To(int64(60 * 60))
	}
}
