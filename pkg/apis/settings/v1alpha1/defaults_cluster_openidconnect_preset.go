// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetDefaults_ClusterOpenIDConnectPresetSpec sets default values for ClusterOpenIDConnectPreset objects.
func SetDefaults_ClusterOpenIDConnectPresetSpec(obj *ClusterOpenIDConnectPresetSpec) {
	if obj.ProjectSelector == nil {
		obj.ProjectSelector = &metav1.LabelSelector{}
	}
}
