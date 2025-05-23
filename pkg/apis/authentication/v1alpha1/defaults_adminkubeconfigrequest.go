// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/utils/ptr"
)

// SetDefaults_AdminKubeconfigRequestSpec sets default values for AdminKubeconfigRequestSpec objects.
func SetDefaults_AdminKubeconfigRequestSpec(obj *AdminKubeconfigRequestSpec) {
	if obj.ExpirationSeconds == nil {
		obj.ExpirationSeconds = ptr.To(int64(60 * 60))
	}
}
