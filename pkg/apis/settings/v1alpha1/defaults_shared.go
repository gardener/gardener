// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// SetDefaults_OpenIDConnectPresetSpec sets default values for OpenIDConnectPresetSpec objects.
func SetDefaults_OpenIDConnectPresetSpec(obj *OpenIDConnectPresetSpec) {
	if obj.ShootSelector == nil {
		obj.ShootSelector = &metav1.LabelSelector{}
	}
}

// SetDefaults_KubeAPIServerOpenIDConnect sets default values for KubeAPIServerOpenIDConnect objects.
func SetDefaults_KubeAPIServerOpenIDConnect(obj *KubeAPIServerOpenIDConnect) {
	if len(obj.SigningAlgs) == 0 {
		obj.SigningAlgs = []string{DefaultSignAlg}
	}

	if obj.UsernameClaim == nil {
		obj.UsernameClaim = ptr.To(DefaultUsernameClaim)
	}
}
