// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_ResourceManagerConfiguration sets defaults for the configuration of the ResourceManagerConfiguration.
func SetDefaults_ResourceManagerConfiguration(obj *ResourceManagerConfiguration) {
	if len(obj.LogLevel) == 0 {
		obj.LogLevel = "info"
	}
	if len(obj.LogFormat) == 0 {
		obj.LogFormat = "json"
	}
}

// SetDefaults_ClientConnection sets defaults for the client connection.
func SetDefaults_ClientConnection(obj *ClientConnection) {
	SetDefaults_ClientConnectionConfiguration(&obj.ClientConnectionConfiguration)

	if obj.CacheResyncPeriod == nil {
		obj.CacheResyncPeriod = &metav1.Duration{Duration: 24 * time.Hour}
	}
}

// SetDefaults_ClientConnectionConfiguration sets defaults for the garden client connection.
func SetDefaults_ClientConnectionConfiguration(obj *componentbaseconfigv1alpha1.ClientConnectionConfiguration) {
	if obj.QPS == 0.0 {
		obj.QPS = 100.0
	}
	if obj.Burst == 0 {
		obj.Burst = 130
	}
}

// SetDefaults_LeaderElectionConfiguration sets defaults for the leader election of the Gardener controller manager.
func SetDefaults_LeaderElectionConfiguration(obj *componentbaseconfigv1alpha1.LeaderElectionConfiguration) {
	if obj.ResourceLock == "" {
		// Don't use a constant from the client-go resourcelock package here (resourcelock is not an API package, pulls
		// in some other dependencies and is thereby not suitable to be used in this API package).
		obj.ResourceLock = "leases"
	}

	componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(obj)

	if obj.ResourceName == "" {
		obj.ResourceName = "gardener-resource-manager"
	}
}

// SetDefaults_ServerConfiguration sets defaults for the server configuration.
func SetDefaults_ServerConfiguration(obj *ServerConfiguration) {
	if obj.Webhooks.Port == 0 {
		obj.Webhooks.Port = 9449
	}

	if obj.HealthProbes == nil {
		obj.HealthProbes = &Server{}
	}
	if obj.HealthProbes.Port == 0 {
		obj.HealthProbes.Port = 8081
	}

	if obj.Metrics == nil {
		obj.Metrics = &Server{}
	}
	if obj.Metrics.Port == 0 {
		obj.Metrics.Port = 8080
	}
}

// SetDefaults_ResourceManagerControllerConfiguration sets defaults for the controller configuration.
func SetDefaults_ResourceManagerControllerConfiguration(obj *ResourceManagerControllerConfiguration) {
	if obj.ClusterID == nil {
		obj.ClusterID = pointer.String("")
	}
	if obj.ResourceClass == nil {
		obj.ResourceClass = pointer.String(DefaultResourceClass)
	}
}

// SetDefaults_KubeletCSRApproverControllerConfig sets defaults for the KubeletCSRApproverControllerConfig object.
func SetDefaults_KubeletCSRApproverControllerConfig(obj *KubeletCSRApproverControllerConfig) {
	if obj.Enabled && obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = pointer.Int(1)
	}
}

// SetDefaults_GarbageCollectorControllerConfig sets defaults for the GarbageCollectorControllerConfig object.
func SetDefaults_GarbageCollectorControllerConfig(obj *GarbageCollectorControllerConfig) {
	if obj.Enabled && obj.SyncPeriod == nil {
		obj.SyncPeriod = &metav1.Duration{Duration: time.Hour}
	}
}

// SetDefaults_NetworkPolicyControllerConfig sets defaults for the NetworkPolicyControllerConfig object.
func SetDefaults_NetworkPolicyControllerConfig(obj *NetworkPolicyControllerConfig) {
	if obj.Enabled && obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = pointer.Int(5)
	}
}

// SetDefaults_HealthControllerConfig sets defaults for the HealthControllerConfig object.
func SetDefaults_HealthControllerConfig(obj *HealthControllerConfig) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = pointer.Int(5)
	}
	if obj.SyncPeriod == nil {
		obj.SyncPeriod = &metav1.Duration{Duration: time.Minute}
	}
}

// SetDefaults_ManagedResourceControllerConfig sets defaults for the ManagedResourceControllerConfig object.
func SetDefaults_ManagedResourceControllerConfig(obj *ManagedResourceControllerConfig) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = pointer.Int(5)
	}
	if obj.SyncPeriod == nil {
		obj.SyncPeriod = &metav1.Duration{Duration: time.Minute}
	}
	if obj.AlwaysUpdate == nil {
		obj.AlwaysUpdate = pointer.Bool(false)
	}
	if obj.ManagedByLabelValue == nil {
		obj.ManagedByLabelValue = pointer.String(resourcesv1alpha1.GardenerManager)
	}
}

// SetDefaults_SecretControllerConfig sets defaults for the SecretControllerConfig object.
func SetDefaults_SecretControllerConfig(obj *SecretControllerConfig) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = pointer.Int(5)
	}
}

// SetDefaults_TokenInvalidatorControllerConfig sets defaults for the TokenInvalidatorControllerConfig object.
func SetDefaults_TokenInvalidatorControllerConfig(obj *TokenInvalidatorControllerConfig) {
	if obj.Enabled && obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = pointer.Int(5)
	}
}

// SetDefaults_TokenRequestorControllerConfig sets defaults for the TokenRequestorControllerConfig object.
func SetDefaults_TokenRequestorControllerConfig(obj *TokenRequestorControllerConfig) {
	if obj.Enabled && obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = pointer.Int(5)
	}
}

// SetDefaults_NodeControllerConfig sets defaults for the NodeControllerConfig object.
func SetDefaults_NodeControllerConfig(obj *NodeControllerConfig) {
	if obj.Enabled {
		if obj.ConcurrentSyncs == nil {
			obj.ConcurrentSyncs = pointer.Int(5)
		}
		if obj.Backoff == nil {
			obj.Backoff = &metav1.Duration{Duration: 10 * time.Second}
		}
	}
}

// SetDefaults_PodSchedulerNameWebhookConfig sets defaults for the PodSchedulerNameWebhookConfig object.
func SetDefaults_PodSchedulerNameWebhookConfig(obj *PodSchedulerNameWebhookConfig) {
	if obj.Enabled && obj.SchedulerName == nil {
		obj.SchedulerName = pointer.String(corev1.DefaultSchedulerName)
	}
}

// SetDefaults_ProjectedTokenMountWebhookConfig sets defaults for the ProjectedTokenMountWebhookConfig object.
func SetDefaults_ProjectedTokenMountWebhookConfig(obj *ProjectedTokenMountWebhookConfig) {
	if obj.Enabled && obj.ExpirationSeconds == nil {
		obj.ExpirationSeconds = pointer.Int64(43200)
	}
}
