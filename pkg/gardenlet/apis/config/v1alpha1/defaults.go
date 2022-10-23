// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_GardenletConfiguration sets defaults for the configuration of the Gardenlet.
func SetDefaults_GardenletConfiguration(obj *GardenletConfiguration) {
	if obj.GardenClientConnection == nil {
		obj.GardenClientConnection = &GardenClientConnection{}
	}

	if obj.SeedClientConnection == nil {
		obj.SeedClientConnection = &SeedClientConnection{}
	}

	if obj.ShootClientConnection == nil {
		obj.ShootClientConnection = &ShootClientConnection{}
	}

	if obj.Controllers == nil {
		obj.Controllers = &GardenletControllerConfiguration{}
	}

	if obj.LeaderElection == nil {
		obj.LeaderElection = &componentbaseconfigv1alpha1.LeaderElectionConfiguration{}
	}

	if obj.LogLevel == "" {
		obj.LogLevel = LogLevelInfo
	}

	if obj.LogFormat == "" {
		obj.LogFormat = LogFormatJSON
	}

	if obj.Server.HealthProbes == nil {
		obj.Server.HealthProbes = &Server{}
	}
	if obj.Server.HealthProbes.Port == 0 {
		obj.Server.HealthProbes.Port = 2728
	}

	if obj.Server.Metrics == nil {
		obj.Server.Metrics = &Server{}
	}
	if obj.Server.Metrics.Port == 0 {
		obj.Server.Metrics.Port = 2729
	}

	if obj.Logging == nil {
		obj.Logging = &Logging{}
	}

	// TODO: consider enabling profiling by default (like in k8s components)

	if obj.SNI == nil {
		obj.SNI = &SNI{}
	}

	if obj.Monitoring == nil {
		obj.Monitoring = &MonitoringConfig{}
	}

	if obj.ETCDConfig == nil {
		obj.ETCDConfig = &ETCDConfig{}
	}

	var defaultSVCName = v1beta1constants.DefaultSNIIngressServiceName
	for i, handler := range obj.ExposureClassHandlers {
		if obj.ExposureClassHandlers[i].SNI == nil {
			obj.ExposureClassHandlers[i].SNI = &SNI{Ingress: &SNIIngress{}}
		}
		if obj.ExposureClassHandlers[i].SNI.Ingress == nil {
			obj.ExposureClassHandlers[i].SNI.Ingress = &SNIIngress{}
		}
		if obj.ExposureClassHandlers[i].SNI.Ingress.Namespace == nil {
			namespaceName := fmt.Sprintf("istio-ingress-handler-%s", handler.Name)
			obj.ExposureClassHandlers[i].SNI.Ingress.Namespace = &namespaceName
		}
		if obj.ExposureClassHandlers[i].SNI.Ingress.ServiceName == nil {
			obj.ExposureClassHandlers[i].SNI.Ingress.ServiceName = &defaultSVCName
		}
		if len(obj.ExposureClassHandlers[i].SNI.Ingress.Labels) == 0 {
			obj.ExposureClassHandlers[i].SNI.Ingress.Labels = map[string]string{
				v1beta1constants.LabelApp:   v1beta1constants.DefaultIngressGatewayAppLabelValue,
				v1beta1constants.GardenRole: v1beta1constants.GardenRoleExposureClassHandler,
			}
		}
	}
}

// SetDefaults_GardenClientConnection sets defaults for the controller objects.
func SetDefaults_GardenClientConnection(obj *GardenClientConnection) {
	if obj.KubeconfigValidity == nil {
		obj.KubeconfigValidity = &KubeconfigValidity{}
	}
}

// SetDefaults_KubeconfigValidity sets defaults for the controller objects.
func SetDefaults_KubeconfigValidity(obj *KubeconfigValidity) {
	if obj.AutoRotationJitterPercentageMin == nil {
		obj.AutoRotationJitterPercentageMin = pointer.Int32(70)
	}
	if obj.AutoRotationJitterPercentageMax == nil {
		obj.AutoRotationJitterPercentageMax = pointer.Int32(90)
	}
}

// SetDefaults_GardenletControllerConfiguration sets defaults for the controller objects.
func SetDefaults_GardenletControllerConfiguration(obj *GardenletControllerConfiguration) {
	if obj.BackupBucket == nil {
		obj.BackupBucket = &BackupBucketControllerConfiguration{}
	}
	if obj.BackupEntry == nil {
		obj.BackupEntry = &BackupEntryControllerConfiguration{}
	}
	if obj.BackupEntryMigration == nil {
		obj.BackupEntryMigration = &BackupEntryMigrationControllerConfiguration{}
	}
	if obj.Bastion == nil {
		obj.Bastion = &BastionControllerConfiguration{}
	}
	if obj.ControllerInstallation == nil {
		obj.ControllerInstallation = &ControllerInstallationControllerConfiguration{}
	}
	if obj.ControllerInstallationCare == nil {
		obj.ControllerInstallationCare = &ControllerInstallationCareControllerConfiguration{}
	}
	if obj.ControllerInstallationRequired == nil {
		obj.ControllerInstallationRequired = &ControllerInstallationRequiredControllerConfiguration{}
	}
	if obj.Seed == nil {
		obj.Seed = &SeedControllerConfiguration{}
	}
	if obj.Shoot == nil {
		obj.Shoot = &ShootControllerConfiguration{}
	}
	if obj.ShootCare == nil {
		obj.ShootCare = &ShootCareControllerConfiguration{}
	}
	if obj.SeedCare == nil {
		obj.SeedCare = &SeedCareControllerConfiguration{}
	}
	if obj.ShootMigration == nil {
		obj.ShootMigration = &ShootMigrationControllerConfiguration{}
	}
	if obj.ShootSecret == nil {
		obj.ShootSecret = &ShootSecretControllerConfiguration{}
	}
	if obj.ShootStateSync == nil {
		obj.ShootStateSync = &ShootStateSyncControllerConfiguration{}
	}
	if obj.SeedAPIServerNetworkPolicy == nil {
		obj.SeedAPIServerNetworkPolicy = &SeedAPIServerNetworkPolicyControllerConfiguration{}
	}
	if obj.ManagedSeed == nil {
		obj.ManagedSeed = &ManagedSeedControllerConfiguration{}
	}
}

// SetDefaults_ClientConnectionConfiguration sets defaults for the client connection objects.
func SetDefaults_ClientConnectionConfiguration(obj *componentbaseconfigv1alpha1.ClientConnectionConfiguration) {
	if obj.QPS == 0.0 {
		obj.QPS = 50.0
	}
	if obj.Burst == 0 {
		obj.Burst = 100
	}
}

// SetDefaults_LeaderElectionConfiguration sets defaults for the leader election of the gardenlet.
func SetDefaults_LeaderElectionConfiguration(obj *componentbaseconfigv1alpha1.LeaderElectionConfiguration) {
	if obj.ResourceLock == "" {
		// Don't use a constant from the client-go resourcelock package here (resourcelock is not an API package, pulls
		// in some other dependencies and is thereby not suitable to be used in this API package).
		obj.ResourceLock = "leases"
	}

	componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(obj)

	if obj.ResourceNamespace == "" {
		obj.ResourceNamespace = GardenletDefaultLockObjectNamespace
	}
	if obj.ResourceName == "" {
		obj.ResourceName = GardenletDefaultLockObjectName
	}
}

// SetDefaults_BackupBucketControllerConfiguration sets defaults for the backup bucket controller.
func SetDefaults_BackupBucketControllerConfiguration(obj *BackupBucketControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}
}

// SetDefaults_BackupEntryControllerConfiguration sets defaults for the backup entry controller.
func SetDefaults_BackupEntryControllerConfiguration(obj *BackupEntryControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}

	if obj.DeletionGracePeriodHours == nil || *obj.DeletionGracePeriodHours < 0 {
		v := DefaultBackupEntryDeletionGracePeriodHours
		obj.DeletionGracePeriodHours = &v
	}
}

// SetDefaults_MonitoringConfig sets the defaults for the monitoring stack.
func SetDefaults_MonitoringConfig(obj *MonitoringConfig) {
	if obj.Shoot == nil {
		obj.Shoot = &ShootMonitoringConfig{}
	}
}

// SetDefaults_ShootMonitoringConfig sets the defaults for the shoot monitoring.
func SetDefaults_ShootMonitoringConfig(obj *ShootMonitoringConfig) {
	if obj.Enabled == nil {
		v := true
		obj.Enabled = &v
	}
}

// SetDefaults_BackupEntryMigrationControllerConfiguration sets defaults for the backup entry migration controller.
func SetDefaults_BackupEntryMigrationControllerConfiguration(obj *BackupEntryMigrationControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := 5
		obj.ConcurrentSyncs = &v
	}

	if obj.SyncPeriod == nil {
		v := metav1.Duration{Duration: time.Minute}
		obj.SyncPeriod = &v
	}

	if obj.GracePeriod == nil {
		v := metav1.Duration{Duration: 10 * time.Minute}
		obj.GracePeriod = &v
	}

	if obj.LastOperationStaleDuration == nil {
		v := metav1.Duration{Duration: 2 * time.Minute}
		obj.LastOperationStaleDuration = &v
	}
}

// SetDefaults_BastionControllerConfiguration sets defaults for the backup bucket controller.
func SetDefaults_BastionControllerConfiguration(obj *BastionControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}
}

// SetDefaults_ControllerInstallationControllerConfiguration sets defaults for the controller installation controller.
func SetDefaults_ControllerInstallationControllerConfiguration(obj *ControllerInstallationControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}
}

// SetDefaults_ControllerInstallationCareControllerConfiguration sets defaults for the controller installation care controller.
func SetDefaults_ControllerInstallationCareControllerConfiguration(obj *ControllerInstallationCareControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}

	if obj.SyncPeriod == nil {
		v := metav1.Duration{Duration: 30 * time.Second}
		obj.SyncPeriod = &v
	}
}

// SetDefaults_ControllerInstallationRequiredControllerConfiguration sets defaults for the ControllerInstallationRequired controller.
func SetDefaults_ControllerInstallationRequiredControllerConfiguration(obj *ControllerInstallationRequiredControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		// The controller actually starts one controller per extension resource per Seed.
		// For one seed that is already 1 * 10 extension resources = 10 workers.
		v := 1
		obj.ConcurrentSyncs = &v
	}
}

// SetDefaults_SeedControllerConfiguration sets defaults for the seed controller.
func SetDefaults_SeedControllerConfiguration(obj *SeedControllerConfiguration) {
	if obj.SyncPeriod == nil {
		v := DefaultControllerSyncPeriod
		obj.SyncPeriod = &v
	}

	if obj.LeaseResyncSeconds == nil {
		obj.LeaseResyncSeconds = pointer.Int32(2)
	}

	if obj.LeaseResyncMissThreshold == nil {
		obj.LeaseResyncMissThreshold = pointer.Int32(10)
	}
}

// SetDefaults_SeedCareControllerConfiguration sets defaults for the seed care controller.
func SetDefaults_SeedCareControllerConfiguration(obj *SeedCareControllerConfiguration) {
	if obj.SyncPeriod == nil {
		v := metav1.Duration{Duration: 30 * time.Second}
		obj.SyncPeriod = &v
	}
}

// SetDefaults_ShootControllerConfiguration sets defaults for the shoot controller.
func SetDefaults_ShootControllerConfiguration(obj *ShootControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}

	if obj.SyncPeriod == nil {
		v := metav1.Duration{Duration: time.Hour}
		obj.SyncPeriod = &v
	}

	if obj.RespectSyncPeriodOverwrite == nil {
		falseVar := false
		obj.RespectSyncPeriodOverwrite = &falseVar
	}

	if obj.ReconcileInMaintenanceOnly == nil {
		falseVar := false
		obj.ReconcileInMaintenanceOnly = &falseVar
	}

	if obj.RetryDuration == nil {
		v := metav1.Duration{Duration: 12 * time.Hour}
		obj.RetryDuration = &v
	}

	if obj.DNSEntryTTLSeconds == nil {
		obj.DNSEntryTTLSeconds = pointer.Int64(120)
	}
}

// SetDefaults_ShootCareControllerConfiguration sets defaults for the shoot care controller.
func SetDefaults_ShootCareControllerConfiguration(obj *ShootCareControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}

	if obj.SyncPeriod == nil {
		v := metav1.Duration{Duration: time.Minute}
		obj.SyncPeriod = &v
	}

	if obj.StaleExtensionHealthChecks == nil {
		v := StaleExtensionHealthChecks{Enabled: true}
		obj.StaleExtensionHealthChecks = &v
	}
}

// SetDefaults_ShootMigrationControllerConfiguration sets defaults for the shoot migration controller.
func SetDefaults_ShootMigrationControllerConfiguration(obj *ShootMigrationControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := 5
		obj.ConcurrentSyncs = &v
	}

	if obj.SyncPeriod == nil {
		v := metav1.Duration{Duration: time.Minute}
		obj.SyncPeriod = &v
	}

	if obj.GracePeriod == nil {
		v := metav1.Duration{Duration: 2 * time.Hour}
		obj.GracePeriod = &v
	}

	if obj.LastOperationStaleDuration == nil {
		v := metav1.Duration{Duration: 10 * time.Minute}
		obj.LastOperationStaleDuration = &v
	}
}

// SetDefaults_StaleExtensionHealthChecks sets defaults for the stale extension health checks.
func SetDefaults_StaleExtensionHealthChecks(obj *StaleExtensionHealthChecks) {
	if obj.Threshold == nil {
		v := metav1.Duration{Duration: 5 * time.Minute}
		obj.Threshold = &v
	}
}

// SetDefaults_ShootSecretControllerConfiguration sets defaults for the shoot secret controller.
func SetDefaults_ShootSecretControllerConfiguration(obj *ShootSecretControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		obj.ConcurrentSyncs = pointer.Int(5)
	}
}

// SetDefaults_ShootStateSyncControllerConfiguration sets defaults for the shoot state controller.
func SetDefaults_ShootStateSyncControllerConfiguration(obj *ShootStateSyncControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		// The controller actually starts one controller per extension resource per Seed.
		// For one seed that is already 1 * 10 extension resources = 10 workers.
		v := 1
		obj.ConcurrentSyncs = &v
	}
}

// SetDefaults_SeedAPIServerNetworkPolicyControllerConfiguration sets defaults for the seed apiserver endpoints controller.
func SetDefaults_SeedAPIServerNetworkPolicyControllerConfiguration(obj *SeedAPIServerNetworkPolicyControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		// only use few workers for each seed, as the API server endpoints should stay the same most of the time.
		v := 3
		obj.ConcurrentSyncs = &v
	}
}

// SetDefaults_ManagedSeedControllerConfiguration sets defaults for the managed seed controller.
func SetDefaults_ManagedSeedControllerConfiguration(obj *ManagedSeedControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}

	if obj.SyncPeriod == nil {
		v := metav1.Duration{Duration: 1 * time.Hour}
		obj.SyncPeriod = &v
	}

	if obj.WaitSyncPeriod == nil {
		v := metav1.Duration{Duration: 15 * time.Second}
		obj.WaitSyncPeriod = &v
	}

	if obj.SyncJitterPeriod == nil {
		v := metav1.Duration{Duration: 5 * time.Minute}
		obj.SyncJitterPeriod = &v
	}

	if obj.JitterUpdates == nil {
		obj.JitterUpdates = pointer.Bool(false)
	}
}

// SetDefaults_SNI sets defaults for SNI.
func SetDefaults_SNI(obj *SNI) {
	if obj.Ingress == nil {
		obj.Ingress = &SNIIngress{}
	}
}

// SetDefaults_SNIIngress sets defaults for SNI ingressgateway.
func SetDefaults_SNIIngress(obj *SNIIngress) {
	var (
		defaultNS      = v1beta1constants.DefaultSNIIngressNamespace
		defaultSVCName = v1beta1constants.DefaultSNIIngressServiceName
	)

	if obj.Namespace == nil {
		obj.Namespace = &defaultNS
	}

	if obj.ServiceName == nil {
		obj.ServiceName = &defaultSVCName
	}

	if obj.Labels == nil {
		obj.Labels = map[string]string{
			v1beta1constants.LabelApp: v1beta1constants.DefaultIngressGatewayAppLabelValue,
			"istio":                   "ingressgateway",
		}
	}
}

// SetDefaults_Logging sets defaults for the Logging stack.
func SetDefaults_Logging(obj *Logging) {
	if obj.Enabled == nil {
		obj.Enabled = pointer.Bool(false)
	}
	if obj.Loki == nil {
		obj.Loki = &Loki{}
	}
	if obj.Loki.Enabled == nil {
		obj.Loki.Enabled = obj.Enabled
	}
	if obj.Loki.Garden == nil {
		obj.Loki.Garden = &GardenLoki{}
	}
	if obj.Loki.Garden.Storage == nil {
		obj.Loki.Garden.Storage = &DefaultCentralLokiStorage
	}
	if obj.ShootEventLogging == nil {
		obj.ShootEventLogging = &ShootEventLogging{}
	}
	if obj.ShootEventLogging.Enabled == nil {
		obj.ShootEventLogging.Enabled = obj.Enabled
	}
}

// SetDefaults_ETCDConfig sets defaults for the ETCD.
func SetDefaults_ETCDConfig(obj *ETCDConfig) {
	if obj.ETCDController == nil {
		obj.ETCDController = &ETCDController{}
	}
	if obj.ETCDController.Workers == nil {
		obj.ETCDController.Workers = pointer.Int64(50)
	}
	if obj.CustodianController == nil {
		obj.CustodianController = &CustodianController{}
	}
	if obj.CustodianController.Workers == nil {
		obj.CustodianController.Workers = pointer.Int64(10)
	}
	if obj.BackupCompactionController == nil {
		obj.BackupCompactionController = &BackupCompactionController{}
	}
	if obj.BackupCompactionController.Workers == nil {
		obj.BackupCompactionController.Workers = pointer.Int64(3)
	}
	if obj.BackupCompactionController.EnableBackupCompaction == nil {
		obj.BackupCompactionController.EnableBackupCompaction = pointer.Bool(false)
	}
	if obj.BackupCompactionController.EventsThreshold == nil {
		obj.BackupCompactionController.EventsThreshold = pointer.Int64(1000000)
	}
}
