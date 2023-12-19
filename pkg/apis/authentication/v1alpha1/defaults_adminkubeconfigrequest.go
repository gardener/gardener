// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/utils/pointer"
)

// SetDefaults_AdminKubeconfigRequestSpec sets default values for AdminKubeconfigRequestSpec objects.
func SetDefaults_AdminKubeconfigRequestSpec(obj *AdminKubeconfigRequestSpec) {
	if obj.ExpirationSeconds == nil {
		obj.ExpirationSeconds = pointer.Int64(60 * 60)
	}
}
