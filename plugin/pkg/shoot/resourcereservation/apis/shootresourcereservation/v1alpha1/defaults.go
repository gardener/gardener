// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// SetDefaults_Configuration sets default values for the Configuration object.
func SetDefaults_Configuration(obj *Configuration) {
	if obj.Selector == nil {
		obj.Selector = &metav1.LabelSelector{}
	}
}

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}
