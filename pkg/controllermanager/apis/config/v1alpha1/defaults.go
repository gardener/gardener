// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_ControllerManagerConfiguration sets defaults for the configuration of the Gardener controller manager.
func SetDefaults_ControllerManagerConfiguration(obj *ControllerManagerConfiguration) {
	if obj.Controllers.Bastion == nil {
		obj.Controllers.Bastion = &BastionControllerConfiguration{}
	}
	if obj.Controllers.Bastion.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.Bastion.ConcurrentSyncs = &v
	}
	if obj.Controllers.Bastion.MaxLifetime == nil {
		obj.Controllers.Bastion.MaxLifetime = &metav1.Duration{Duration: 24 * time.Hour}
	}

	if obj.Controllers.CertificateSigningRequest == nil {
		obj.Controllers.CertificateSigningRequest = &CertificateSigningRequestControllerConfiguration{}
	}

	if obj.Controllers.CloudProfile == nil {
		obj.Controllers.CloudProfile = &CloudProfileControllerConfiguration{}
	}
	if obj.Controllers.CloudProfile.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.CloudProfile.ConcurrentSyncs = &v
	}

	if obj.Controllers.ControllerDeployment == nil {
		obj.Controllers.ControllerDeployment = &ControllerDeploymentControllerConfiguration{}
	}
	if obj.Controllers.ControllerDeployment.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.ControllerDeployment.ConcurrentSyncs = &v
	}

	if obj.Controllers.ControllerRegistration == nil {
		obj.Controllers.ControllerRegistration = &ControllerRegistrationControllerConfiguration{}
	}
	if obj.Controllers.ControllerRegistration.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.ControllerRegistration.ConcurrentSyncs = &v
	}

	if obj.Controllers.ExposureClass == nil {
		obj.Controllers.ExposureClass = &ExposureClassControllerConfiguration{}
	}
	if obj.Controllers.ExposureClass.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.ExposureClass.ConcurrentSyncs = &v
	}

	if obj.Controllers.Project == nil {
		obj.Controllers.Project = &ProjectControllerConfiguration{}
	}
	if obj.Controllers.Project.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.Project.ConcurrentSyncs = &v
	}
	if obj.Controllers.Project.MinimumLifetimeDays == nil {
		v := 30
		obj.Controllers.Project.MinimumLifetimeDays = &v
	}
	if obj.Controllers.Project.StaleGracePeriodDays == nil {
		v := 14
		obj.Controllers.Project.StaleGracePeriodDays = &v
	}
	if obj.Controllers.Project.StaleExpirationTimeDays == nil {
		v := 90
		obj.Controllers.Project.StaleExpirationTimeDays = &v
	}
	if obj.Controllers.Project.StaleSyncPeriod == nil {
		obj.Controllers.Project.StaleSyncPeriod = &metav1.Duration{
			Duration: 12 * time.Hour,
		}
	}
	for i, quota := range obj.Controllers.Project.Quotas {
		if quota.ProjectSelector == nil {
			obj.Controllers.Project.Quotas[i].ProjectSelector = &metav1.LabelSelector{}
		}
	}

	if obj.Controllers.Quota == nil {
		obj.Controllers.Quota = &QuotaControllerConfiguration{}
	}
	if obj.Controllers.Quota.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.Quota.ConcurrentSyncs = &v
	}

	if obj.Controllers.SecretBinding == nil {
		obj.Controllers.SecretBinding = &SecretBindingControllerConfiguration{}
	}
	if obj.Controllers.SecretBinding.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.SecretBinding.ConcurrentSyncs = &v
	}

	if obj.Controllers.Seed == nil {
		obj.Controllers.Seed = &SeedControllerConfiguration{}
	}

	if obj.Controllers.SeedExtensionsCheck == nil {
		obj.Controllers.SeedExtensionsCheck = &SeedExtensionsCheckControllerConfiguration{}
	}

	if obj.Controllers.SeedBackupBucketsCheck == nil {
		obj.Controllers.SeedBackupBucketsCheck = &SeedBackupBucketsCheckControllerConfiguration{}
	}

	if obj.Controllers.ShootMaintenance.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.ShootMaintenance.ConcurrentSyncs = &v
	}
	if obj.Controllers.ShootMaintenance.EnableShootControlPlaneRestarter == nil {
		b := true
		obj.Controllers.ShootMaintenance.EnableShootControlPlaneRestarter = &b
	}

	if obj.Controllers.ShootQuota == nil {
		obj.Controllers.ShootQuota = &ShootQuotaControllerConfiguration{}
	}
	if obj.Controllers.ShootQuota.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.ShootQuota.ConcurrentSyncs = &v
	}
	if obj.Controllers.ShootQuota.SyncPeriod == nil {
		obj.Controllers.ShootQuota.SyncPeriod = &metav1.Duration{
			Duration: 60 * time.Minute,
		}
	}

	if obj.Controllers.ShootReference == nil {
		obj.Controllers.ShootReference = &ShootReferenceControllerConfiguration{}
	}
	if obj.Controllers.ShootReference.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.ShootReference.ConcurrentSyncs = &v
	}

	if obj.Controllers.ShootRetry == nil {
		obj.Controllers.ShootRetry = &ShootRetryControllerConfiguration{}
	}
	if obj.Controllers.ShootRetry.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.ShootRetry.ConcurrentSyncs = &v
	}

	if obj.Controllers.ShootConditions == nil {
		obj.Controllers.ShootConditions = &ShootConditionsControllerConfiguration{}
	}
	if obj.Controllers.ShootConditions.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.ShootConditions.ConcurrentSyncs = &v
	}

	if obj.Controllers.ShootStatusLabel == nil {
		obj.Controllers.ShootStatusLabel = &ShootStatusLabelControllerConfiguration{}
	}
	if obj.Controllers.ShootStatusLabel.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.ShootStatusLabel.ConcurrentSyncs = &v
	}

	if obj.Controllers.ManagedSeedSet == nil {
		obj.Controllers.ManagedSeedSet = &ManagedSeedSetControllerConfiguration{
			SyncPeriod: metav1.Duration{
				Duration: 30 * time.Minute,
			},
		}
	}
	if obj.Controllers.ManagedSeedSet.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.Controllers.ManagedSeedSet.ConcurrentSyncs = &v
	}

	if obj.LogLevel == "" {
		obj.LogLevel = LogLevelInfo
	}

	if obj.LogFormat == "" {
		obj.LogFormat = LogFormatJSON
	}

	if obj.LeaderElection == nil {
		obj.LeaderElection = &componentbaseconfigv1alpha1.LeaderElectionConfiguration{}
	}

	if obj.Server.HealthProbes == nil {
		obj.Server.HealthProbes = &Server{}
	}
	if obj.Server.HealthProbes.Port == 0 {
		obj.Server.HealthProbes.Port = 2718
	}

	if obj.Server.Metrics == nil {
		obj.Server.Metrics = &Server{}
	}
	if obj.Server.Metrics.Port == 0 {
		obj.Server.Metrics.Port = 2719
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

// SetDefaults_LeaderElectionConfiguration sets defaults for the leader election of the Gardener controller manager.
func SetDefaults_LeaderElectionConfiguration(obj *componentbaseconfigv1alpha1.LeaderElectionConfiguration) {
	if obj.ResourceLock == "" {
		// Don't use a constant from the client-go resourcelock package here (resourcelock is not an API package, pulls
		// in some other dependencies and is thereby not suitable to be used in this API package).
		obj.ResourceLock = "leases"
	}

	componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(obj)

	if obj.ResourceNamespace == "" {
		obj.ResourceNamespace = ControllerManagerDefaultLockObjectNamespace
	}
	if obj.ResourceName == "" {
		obj.ResourceName = ControllerManagerDefaultLockObjectName
	}
}

// SetDefaults_EventControllerConfiguration sets defaults for the EventControllerConfiguration.
func SetDefaults_EventControllerConfiguration(obj *EventControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}
	if obj.TTLNonShootEvents == nil {
		obj.TTLNonShootEvents = &metav1.Duration{Duration: 1 * time.Hour}
	}
}

// SetDefaults_ShootRetryControllerConfiguration sets defaults for the ShootRetryControllerConfiguration.
func SetDefaults_ShootRetryControllerConfiguration(obj *ShootRetryControllerConfiguration) {
	if obj.RetryPeriod == nil {
		obj.RetryPeriod = &metav1.Duration{Duration: 10 * time.Minute}
	}
	if obj.RetryJitterPeriod == nil {
		obj.RetryJitterPeriod = &metav1.Duration{Duration: 5 * time.Minute}
	}
}

// SetDefaults_ManagedSeedSetControllerConfiguration sets defaults for the given ManagedSeedSetControllerConfiguration.
func SetDefaults_ManagedSeedSetControllerConfiguration(obj *ManagedSeedSetControllerConfiguration) {
	if obj.MaxShootRetries == nil {
		v := 3
		obj.MaxShootRetries = &v
	}
}

// SetDefaults_ShootHibernationControllerConfiguration sets defaults for the given ShootHibernationControllerConfiguration.
func SetDefaults_ShootHibernationControllerConfiguration(obj *ShootHibernationControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}
	if obj.TriggerDeadlineDuration == nil {
		obj.TriggerDeadlineDuration = &metav1.Duration{Duration: 2 * time.Hour}
	}
}

// SetDefaults_SeedExtensionsCheckControllerConfiguration sets defaults for the given SeedExtensionsCheckControllerConfiguration.
func SetDefaults_SeedExtensionsCheckControllerConfiguration(obj *SeedExtensionsCheckControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}
	if obj.SyncPeriod == nil {
		v := metav1.Duration{Duration: 30 * time.Second}
		obj.SyncPeriod = &v
	}
}

// SetDefaults_SeedBackupBucketsCheckControllerConfiguration sets defaults for the given SeedBackupBucketsCheckControllerConfiguration.
func SetDefaults_SeedBackupBucketsCheckControllerConfiguration(obj *SeedBackupBucketsCheckControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}
	if obj.SyncPeriod == nil {
		obj.SyncPeriod = &metav1.Duration{Duration: 30 * time.Second}
	}
}

// SetDefaults_CertificateSigningRequestControllerConfiguration sets defaults for the given CertificateSigningRequestControllerConfiguration.
func SetDefaults_CertificateSigningRequestControllerConfiguration(obj *CertificateSigningRequestControllerConfiguration) {
	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}
}

// SetDefaults_SeedControllerConfiguration sets defaults for the given SeedControllerConfiguration.
func SetDefaults_SeedControllerConfiguration(obj *SeedControllerConfiguration) {
	if obj.SyncPeriod == nil {
		obj.SyncPeriod = &metav1.Duration{Duration: 10 * time.Second}
	}

	if obj.ConcurrentSyncs == nil {
		v := DefaultControllerConcurrentSyncs
		obj.ConcurrentSyncs = &v
	}

	if obj.MonitorPeriod == nil {
		v := metav1.Duration{Duration: 40 * time.Second}
		obj.MonitorPeriod = &v
	}

	if obj.ShootMonitorPeriod == nil {
		v := metav1.Duration{Duration: 5 * obj.MonitorPeriod.Duration}
		obj.ShootMonitorPeriod = &v
	}
}
