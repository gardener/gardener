// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
)

// SetDefaults_OperatorConfiguration sets defaults for the configuration of the Gardener operator.
func SetDefaults_OperatorConfiguration(obj *OperatorConfiguration) {
	if obj.LogLevel == "" {
		obj.LogLevel = logger.InfoLevel
	}
	if obj.LogFormat == "" {
		obj.LogFormat = logger.FormatJSON
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

// SetDefaults_LeaderElectionConfiguration sets defaults for the leader election of the Gardener operator.
func SetDefaults_LeaderElectionConfiguration(obj *componentbaseconfigv1alpha1.LeaderElectionConfiguration) {
	if obj.ResourceLock == "" {
		// Don't use a constant from the client-go resourcelock package here (resourcelock is not an API package, pulls
		// in some other dependencies and is thereby not suitable to be used in this API package).
		obj.ResourceLock = "leases"
	}

	componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(obj)

	if obj.ResourceNamespace == "" {
		obj.ResourceNamespace = DefaultLockObjectNamespace
	}
	if obj.ResourceName == "" {
		obj.ResourceName = DefaultLockObjectName
	}
}

// SetDefaults_ServerConfiguration sets defaults for the server configuration.
func SetDefaults_ServerConfiguration(obj *ServerConfiguration) {
	if obj.Webhooks.Port == 0 {
		obj.Webhooks.Port = 2750
	}

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

// SetDefaults_GardenControllerConfig sets defaults for the GardenControllerConfig object.
func SetDefaults_GardenControllerConfig(obj *GardenControllerConfig) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = ptr.To(1)
	}
	if obj.SyncPeriod == nil {
		obj.SyncPeriod = &metav1.Duration{Duration: time.Hour}
	}
	if obj.ETCDConfig == nil {
		obj.ETCDConfig = &gardenletconfigv1alpha1.ETCDConfig{}
	}

	gardenletconfigv1alpha1.SetDefaults_ETCDConfig(obj.ETCDConfig)
	gardenletconfigv1alpha1.SetDefaults_ETCDController(obj.ETCDConfig.ETCDController)
	gardenletconfigv1alpha1.SetDefaults_CustodianController(obj.ETCDConfig.CustodianController)
	gardenletconfigv1alpha1.SetDefaults_BackupCompactionController(obj.ETCDConfig.BackupCompactionController)
}

// SetDefaults_GardenCareControllerConfiguration sets defaults for the GardenCareControllerConfiguration object.
func SetDefaults_GardenCareControllerConfiguration(obj *GardenCareControllerConfiguration) {
	if obj.SyncPeriod == nil {
		obj.SyncPeriod = &metav1.Duration{Duration: time.Minute}
	}
}

// SetDefaults_GardenletDeployerControllerConfig sets defaults for the GardenletDeployerControllerConfig object.
func SetDefaults_GardenletDeployerControllerConfig(obj *GardenletDeployerControllerConfig) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = ptr.To(1)
	}
}

// SetDefaults_ExtensionControllerConfiguration sets defaults for the ExtensionControllerConfiguration object.
func SetDefaults_ExtensionControllerConfiguration(obj *ExtensionControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = ptr.To(5)
	}
}

// SetDefaults_ExtensionCareControllerConfiguration sets defaults for the ExtensionCareControllerConfiguration object.
func SetDefaults_ExtensionCareControllerConfiguration(obj *ExtensionCareControllerConfiguration) {
	if obj.SyncPeriod == nil {
		obj.SyncPeriod = &metav1.Duration{Duration: time.Minute}
	}
}

// SetDefaults_ExtensionRequiredRuntimeControllerConfiguration sets defaults for the ExtensionRequiredControllerRuntimeConfiguration object.
func SetDefaults_ExtensionRequiredRuntimeControllerConfiguration(obj *ExtensionRequiredRuntimeControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = ptr.To(5)
	}
}

// SetDefaults_ExtensionRequiredVirtualControllerConfiguration sets defaults for the ExtensionRequiredControllerVirtualConfiguration object.
func SetDefaults_ExtensionRequiredVirtualControllerConfiguration(obj *ExtensionRequiredVirtualControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = ptr.To(5)
	}
}
