// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// SetDefaults_ResourceManagerConfiguration sets defaults for the configuration of the ResourceManagerConfiguration.
func SetDefaults_ResourceManagerConfiguration(obj *ResourceManagerConfiguration) {
	if obj.TargetClientConnection == nil {
		obj.TargetClientConnection = &ClientConnection{}
	}
	if len(obj.LogLevel) == 0 {
		obj.LogLevel = "info"
	}
	if len(obj.LogFormat) == 0 {
		obj.LogFormat = "json"
	}
}

// SetDefaults_ClientConnection sets defaults for the client connection.
func SetDefaults_ClientConnection(obj *ClientConnection) {
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
		obj.ClusterID = ptr.To("")
	}
	if obj.ResourceClass == nil {
		obj.ResourceClass = ptr.To(DefaultResourceClass)
	}
}

// SetDefaults_CSRApproverControllerConfig sets defaults for the CSRApproverControllerConfig object.
func SetDefaults_CSRApproverControllerConfig(obj *CSRApproverControllerConfig) {
	if obj.Enabled && obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = ptr.To(1)
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
		obj.ConcurrentSyncs = ptr.To(5)
	}
}

// SetDefaults_HealthControllerConfig sets defaults for the HealthControllerConfig object.
func SetDefaults_HealthControllerConfig(obj *HealthControllerConfig) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = ptr.To(5)
	}
	if obj.SyncPeriod == nil {
		obj.SyncPeriod = &metav1.Duration{Duration: time.Minute}
	}
}

// SetDefaults_ManagedResourceControllerConfig sets defaults for the ManagedResourceControllerConfig object.
func SetDefaults_ManagedResourceControllerConfig(obj *ManagedResourceControllerConfig) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = ptr.To(5)
	}
	if obj.SyncPeriod == nil {
		obj.SyncPeriod = &metav1.Duration{Duration: time.Minute}
	}
	if obj.AlwaysUpdate == nil {
		obj.AlwaysUpdate = ptr.To(false)
	}
	if obj.ManagedByLabelValue == nil {
		obj.ManagedByLabelValue = ptr.To(resourcesv1alpha1.GardenerManager)
	}
}

// SetDefaults_TokenRequestorControllerConfig sets defaults for the TokenRequestorControllerConfig object.
func SetDefaults_TokenRequestorControllerConfig(obj *TokenRequestorControllerConfig) {
	if obj.Enabled && obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = ptr.To(5)
	}
}

// SetDefaults_NodeCriticalComponentsControllerConfig sets defaults for the NodeCriticalComponentsControllerConfig object.
func SetDefaults_NodeCriticalComponentsControllerConfig(obj *NodeCriticalComponentsControllerConfig) {
	if obj.Enabled {
		if obj.ConcurrentSyncs == nil {
			obj.ConcurrentSyncs = ptr.To(5)
		}
		if obj.Backoff == nil {
			obj.Backoff = &metav1.Duration{Duration: 10 * time.Second}
		}
	}
}

// SetDefaults_NodeAgentReconciliationDelayControllerConfig sets defaults for the NodeAgentReconciliationDelayControllerConfig object.
func SetDefaults_NodeAgentReconciliationDelayControllerConfig(obj *NodeAgentReconciliationDelayControllerConfig) {
	if obj.Enabled {
		if obj.MinDelay == nil {
			obj.MinDelay = &metav1.Duration{}
		}
		if obj.MaxDelay == nil {
			obj.MaxDelay = &metav1.Duration{Duration: 5 * time.Minute}
		}
	}
}

// SetDefaults_PodSchedulerNameWebhookConfig sets defaults for the PodSchedulerNameWebhookConfig object.
func SetDefaults_PodSchedulerNameWebhookConfig(obj *PodSchedulerNameWebhookConfig) {
	if obj.Enabled && obj.SchedulerName == nil {
		obj.SchedulerName = ptr.To(corev1.DefaultSchedulerName)
	}
}

// SetDefaults_ProjectedTokenMountWebhookConfig sets defaults for the ProjectedTokenMountWebhookConfig object.
func SetDefaults_ProjectedTokenMountWebhookConfig(obj *ProjectedTokenMountWebhookConfig) {
	if obj.Enabled && obj.ExpirationSeconds == nil {
		obj.ExpirationSeconds = ptr.To[int64](43200)
	}
}
