// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_AdmissionControllerConfiguration sets defaults for the configuration of the Gardener admission controller.
func SetDefaults_AdmissionControllerConfiguration(obj *AdmissionControllerConfiguration) {
	if len(obj.LogLevel) == 0 {
		obj.LogLevel = "info"
	}
	if len(obj.Server.HTTPS.BindAddress) == 0 {
		obj.Server.HTTPS.BindAddress = "0.0.0.0"
	}
	if obj.Server.HTTPS.Port == 0 {
		obj.Server.HTTPS.Port = 2721
	}

	if obj.Server.ResourceAdmissionConfiguration == nil {
		obj.Server.ResourceAdmissionConfiguration = &ResourceAdmissionConfiguration{}
	}

	resourceAdmission := obj.Server.ResourceAdmissionConfiguration
	for i, subject := range resourceAdmission.UnrestrictedSubjects {
		if (subject.Kind == rbacv1.UserKind || subject.Kind == rbacv1.GroupKind) && subject.APIGroup == "" {
			resourceAdmission.UnrestrictedSubjects[i].APIGroup = rbacv1.GroupName
		}
	}
}
