// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_AdmissionControllerConfiguration sets defaults for the configuration of the Gardener admission controller.
func SetDefaults_AdmissionControllerConfiguration(obj *AdmissionControllerConfiguration) {
	if len(obj.LogLevel) == 0 {
		obj.LogLevel = "info"
	}
	if len(obj.LogFormat) == 0 {
		obj.LogFormat = "json"
	}
	if obj.Server.Webhooks.Port == 0 {
		obj.Server.Webhooks.Port = 2721
	}

	if obj.Server.ResourceAdmissionConfiguration == nil {
		obj.Server.ResourceAdmissionConfiguration = &ResourceAdmissionConfiguration{}
	}

	if obj.Server.HealthProbes == nil {
		obj.Server.HealthProbes = &Server{}
	}
	if obj.Server.HealthProbes.Port == 0 {
		obj.Server.HealthProbes.Port = 2722
	}

	if obj.Server.Metrics == nil {
		obj.Server.Metrics = &Server{}
	}
	if obj.Server.Metrics.Port == 0 {
		obj.Server.Metrics.Port = 2723
	}

	resourceAdmission := obj.Server.ResourceAdmissionConfiguration
	for i, subject := range resourceAdmission.UnrestrictedSubjects {
		if (subject.Kind == rbacv1.UserKind || subject.Kind == rbacv1.GroupKind) && subject.APIGroup == "" {
			resourceAdmission.UnrestrictedSubjects[i].APIGroup = rbacv1.GroupName
		}
	}
}

// SetDefaults_ClientConnectionConfiguration sets defaults for the garden client connection.
func SetDefaults_ClientConnectionConfiguration(obj *componentbaseconfigv1alpha1.ClientConnectionConfiguration) {
	if obj.QPS == 0.0 {
		obj.QPS = 50.0
	}
	if obj.Burst == 0 {
		obj.Burst = 100
	}
}
