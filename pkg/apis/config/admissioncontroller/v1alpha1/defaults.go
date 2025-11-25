// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

// SetDefaults_AdmissionControllerConfiguration sets defaults for the configuration of the Gardener admission controller.
func SetDefaults_AdmissionControllerConfiguration(obj *AdmissionControllerConfiguration) {
	if len(obj.LogLevel) == 0 {
		obj.LogLevel = "info"
	}

	if len(obj.LogFormat) == 0 {
		obj.LogFormat = "json"
	}
}

// SetDefaults_ServerConfiguration sets defaults for the configuration of the server
func SetDefaults_ServerConfiguration(obj *ServerConfiguration) {
	if obj.Webhooks.Port == 0 {
		obj.Webhooks.Port = 2721
	}

	if obj.ResourceAdmissionConfiguration == nil {
		obj.ResourceAdmissionConfiguration = &ResourceAdmissionConfiguration{}
	}

	if obj.HealthProbes == nil {
		obj.HealthProbes = &Server{}
	}

	if obj.HealthProbes.Port == 0 {
		obj.HealthProbes.Port = 2722
	}

	if obj.Metrics == nil {
		obj.Metrics = &Server{}
	}

	if obj.Metrics.Port == 0 {
		obj.Metrics.Port = 2723
	}
}

// SetDefaults_ResourceAdmissionConfiguration sets defaults for the resource admission configuration.
func SetDefaults_ResourceAdmissionConfiguration(obj *ResourceAdmissionConfiguration) {
	for i, subject := range obj.UnrestrictedSubjects {
		if (subject.Kind == rbacv1.UserKind || subject.Kind == rbacv1.GroupKind) && subject.APIGroup == "" {
			obj.UnrestrictedSubjects[i].APIGroup = rbacv1.GroupName
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
