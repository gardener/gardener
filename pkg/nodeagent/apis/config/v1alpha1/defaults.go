// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"

	"github.com/gardener/gardener/pkg/logger"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_NodeAgentConfiguration sets defaults for the configuration of the Gardener Node Agent.
func SetDefaults_NodeAgentConfiguration(obj *NodeAgentConfiguration) {
	if obj.LogLevel == "" {
		obj.LogLevel = logger.InfoLevel
	}
	if obj.LogFormat == "" {
		obj.LogFormat = logger.FormatJSON
	}
}

// SetDefaults_OperatingSystemConfigControllerConfig sets defaults for the OperatingSystemConfigControllerConfig object.
func SetDefaults_OperatingSystemConfigControllerConfig(obj *OperatingSystemConfigControllerConfig) {
	if obj.SyncPeriod == nil {
		obj.SyncPeriod = &metav1.Duration{Duration: 10 * time.Minute}
	}

	if obj.SyncJitterPeriod == nil {
		obj.SyncJitterPeriod = &metav1.Duration{Duration: 5 * time.Minute}
	}
}

// SetDefaults_TokenControllerConfig sets defaults for the TokenControllerConfig object.
func SetDefaults_TokenControllerConfig(obj *TokenControllerConfig) {
	if obj.SyncPeriod == nil {
		obj.SyncPeriod = &metav1.Duration{Duration: time.Hour}
	}
}

// SetDefaults_ClientConnectionConfiguration sets defaults for the garden client connection.
func SetDefaults_ClientConnectionConfiguration(obj *componentbaseconfigv1alpha1.ClientConnectionConfiguration) {
	componentbaseconfigv1alpha1.RecommendedDefaultClientConnectionConfiguration(obj)
}

// SetDefaults_ServerConfiguration sets defaults for the server configuration.
func SetDefaults_ServerConfiguration(obj *ServerConfiguration) {
	if obj.HealthProbes == nil {
		obj.HealthProbes = &Server{}
	}
	if obj.HealthProbes.Port == 0 {
		obj.HealthProbes.Port = 2751
	}

	if obj.Metrics == nil {
		obj.Metrics = &Server{}
	}
	if obj.Metrics.Port == 0 {
		obj.Metrics.Port = 2752
	}
}
