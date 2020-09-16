// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_OpenIDConnectPreset sets default values for OpenIDConnectPreset objects.
func SetDefaults_OpenIDConnectPreset(obj *OpenIDConnectPreset) {
	if obj.Spec.ShootSelector == nil {
		obj.Spec.ShootSelector = &metav1.LabelSelector{}
	}
	setDefaultServerSpec(&obj.Spec.Server)
}

// SetDefaults_ClusterOpenIDConnectPreset sets default values for ClusterOpenIDConnectPreset objects.
func SetDefaults_ClusterOpenIDConnectPreset(obj *ClusterOpenIDConnectPreset) {
	if obj.Spec.ShootSelector == nil {
		obj.Spec.ShootSelector = &metav1.LabelSelector{}
	}

	if obj.Spec.ProjectSelector == nil {
		obj.Spec.ProjectSelector = &metav1.LabelSelector{}
	}
	setDefaultServerSpec(&obj.Spec.Server)
}

func setDefaultServerSpec(spec *KubeAPIServerOpenIDConnect) {
	if len(spec.SigningAlgs) == 0 {
		spec.SigningAlgs = []string{DefaultSignAlg}
	}

	if spec.UsernameClaim == nil {
		usernameClaim := DefaultUsernameClaim
		spec.UsernameClaim = &usernameClaim
	}
}
