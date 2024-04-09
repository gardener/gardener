// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
