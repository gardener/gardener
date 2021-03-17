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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
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
	if obj.Controllers.BackupBucket == nil {
		obj.Controllers.BackupBucket = &BackupBucketControllerConfiguration{}
	}
	if obj.Controllers.BackupEntry == nil {
		obj.Controllers.BackupEntry = &BackupEntryControllerConfiguration{}
	}
	if obj.Controllers.ControllerInstallation == nil {
		obj.Controllers.ControllerInstallation = &ControllerInstallationControllerConfiguration{}
	}
	if obj.Controllers.ControllerInstallationCare == nil {
		obj.Controllers.ControllerInstallationCare = &ControllerInstallationCareControllerConfiguration{}
	}
	if obj.Controllers.ControllerInstallationRequired == nil {
		obj.Controllers.ControllerInstallationRequired = &ControllerInstallationRequiredControllerConfiguration{}
	}
	if obj.Controllers.Seed == nil {
		obj.Controllers.Seed = &SeedControllerConfiguration{}
	}
	if obj.Controllers.Shoot == nil {
		obj.Controllers.Shoot = &ShootControllerConfiguration{}
	}
	if obj.Controllers.ShootCare == nil {
		obj.Controllers.ShootCare = &ShootCareControllerConfiguration{}
	}
	if obj.Controllers.ShootStateSync == nil {
		obj.Controllers.ShootStateSync = &ShootStateSyncControllerConfiguration{}
	}
	if obj.Controllers.SeedAPIServerNetworkPolicy == nil {
		obj.Controllers.SeedAPIServerNetworkPolicy = &SeedAPIServerNetworkPolicyControllerConfiguration{}
	}
	if obj.Controllers.ManagedSeed == nil {
		obj.Controllers.ManagedSeed = &ManagedSeedControllerConfiguration{}
	}

	if obj.LeaderElection == nil {
		obj.LeaderElection = &LeaderElectionConfiguration{}
	}

	if obj.LogLevel == nil {
		v := DefaultLogLevel
		obj.LogLevel = &v
	}

	if obj.KubernetesLogLevel == nil {
		v := DefaultKubernetesLogLevel
		obj.KubernetesLogLevel = &v
	}

	if obj.Server == nil {
		obj.Server = &ServerConfiguration{}
	}
	if len(obj.Server.HTTPS.BindAddress) == 0 {
		obj.Server.HTTPS.BindAddress = "0.0.0.0"
	}
	if obj.Server.HTTPS.Port == 0 {
		obj.Server.HTTPS.Port = 2720
	}

	if obj.SNI == nil {
		obj.SNI = &SNI{}
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
func SetDefaults_LeaderElectionConfiguration(obj *LeaderElectionConfiguration) {
	if obj.ResourceLock == "" {
		obj.ResourceLock = resourcelock.LeasesResourceLock
	}

	componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(&obj.LeaderElectionConfiguration)

	if obj.LockObjectNamespace == nil {
		v := GardenletDefaultLockObjectNamespace
		obj.LockObjectNamespace = &v
	}
	if obj.LockObjectName == nil {
		v := GardenletDefaultLockObjectName
		obj.LockObjectName = &v
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
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}

	if obj.SyncPeriod == nil {
		v := DefaultControllerSyncPeriod
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
		obj.DNSEntryTTLSeconds = pointer.Int64Ptr(120)
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

// SetDefaults_StaleExtensionHealthChecks sets defaults for the stale extension health checks.
func SetDefaults_StaleExtensionHealthChecks(obj *StaleExtensionHealthChecks) {
	if obj.Threshold == nil {
		v := metav1.Duration{Duration: 5 * time.Minute}
		obj.Threshold = &v
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

	if obj.SyncPeriod == nil {
		v := metav1.Duration{Duration: time.Minute}
		obj.SyncPeriod = &v
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

	if obj.SyncJitterPeriod == nil {
		v := metav1.Duration{Duration: 5 * time.Minute}
		obj.SyncJitterPeriod = &v
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
		defaultNS      = DefaultSNIIngresNamespace
		defaultSVCName = DefaultSNIIngresServiceName
	)

	if obj.Namespace == nil {
		obj.Namespace = &defaultNS
	}

	if obj.ServiceName == nil {
		obj.ServiceName = &defaultSVCName
	}

	if obj.Labels == nil {
		obj.Labels = map[string]string{"istio": "ingressgateway"}
	}
}
