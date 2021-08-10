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

	"github.com/gardener/gardener/pkg/logger"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_ControllerManagerConfiguration sets defaults for the configuration of the Gardener controller manager.
func SetDefaults_ControllerManagerConfiguration(obj *ControllerManagerConfiguration) {
	if len(obj.Server.HTTP.BindAddress) == 0 {
		obj.Server.HTTP.BindAddress = "0.0.0.0"
	}
	if obj.Server.HTTP.Port == 0 {
		obj.Server.HTTP.Port = 2718
	}
	if len(obj.Server.HTTPS.BindAddress) == 0 {
		obj.Server.HTTPS.BindAddress = "0.0.0.0"
	}
	if obj.Server.HTTPS.Port == 0 {
		obj.Server.HTTPS.Port = 2719
	}

	if obj.Controllers.Bastion == nil {
		obj.Controllers.Bastion = &BastionControllerConfiguration{}
	}
	if obj.Controllers.Bastion.ConcurrentSyncs == 0 {
		obj.Controllers.Bastion.ConcurrentSyncs = 5
	}
	if obj.Controllers.Bastion.MaxLifetime == nil {
		obj.Controllers.Bastion.MaxLifetime = &metav1.Duration{Duration: 24 * time.Hour}
	}

	if obj.Controllers.CloudProfile == nil {
		obj.Controllers.CloudProfile = &CloudProfileControllerConfiguration{
			ConcurrentSyncs: 5,
		}
	}

	if obj.Controllers.ControllerDeployment == nil {
		obj.Controllers.ControllerDeployment = &ControllerDeploymentControllerConfiguration{
			ConcurrentSyncs: 5,
		}
	}

	if obj.Controllers.ControllerRegistration == nil {
		obj.Controllers.ControllerRegistration = &ControllerRegistrationControllerConfiguration{
			ConcurrentSyncs: 5,
		}
	}

	if obj.Controllers.ExposureClass == nil {
		obj.Controllers.ExposureClass = &ExposureClassControllerConfiguration{
			ConcurrentSyncs: 5,
		}
	}

	if obj.Controllers.Project == nil {
		obj.Controllers.Project = &ProjectControllerConfiguration{
			ConcurrentSyncs: 5,
		}
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
		obj.Controllers.Quota = &QuotaControllerConfiguration{
			ConcurrentSyncs: 5,
		}
	}

	if obj.Controllers.Plant == nil {
		obj.Controllers.Plant = &PlantControllerConfiguration{
			ConcurrentSyncs: 5,
			SyncPeriod: metav1.Duration{
				Duration: 30 * time.Second,
			},
		}
	}

	if obj.Controllers.SecretBinding == nil {
		obj.Controllers.SecretBinding = &SecretBindingControllerConfiguration{
			ConcurrentSyncs: 5,
		}
	}

	if obj.Controllers.ShootReference == nil {
		obj.Controllers.ShootReference = &ShootReferenceControllerConfiguration{
			ConcurrentSyncs: 5,
		}
	}

	if obj.Controllers.ShootRetry == nil {
		obj.Controllers.ShootRetry = &ShootRetryControllerConfiguration{
			ConcurrentSyncs: 5,
		}
	}

	if obj.Controllers.Seed == nil {
		obj.Controllers.Seed = &SeedControllerConfiguration{
			ConcurrentSyncs: 5,
			SyncPeriod: metav1.Duration{
				Duration: 30 * time.Second,
			},
		}
	}
	if obj.Controllers.Seed.MonitorPeriod == nil {
		v := metav1.Duration{Duration: 40 * time.Second}
		obj.Controllers.Seed.MonitorPeriod = &v
	}
	if obj.Controllers.Seed.ShootMonitorPeriod == nil {
		v := metav1.Duration{Duration: 5 * obj.Controllers.Seed.MonitorPeriod.Duration}
		obj.Controllers.Seed.ShootMonitorPeriod = &v
	}

	if obj.Controllers.ManagedSeedSet == nil {
		obj.Controllers.ManagedSeedSet = &ManagedSeedSetControllerConfiguration{
			ConcurrentSyncs: 5,
			SyncPeriod: metav1.Duration{
				Duration: 30 * time.Minute,
			},
		}
	}

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
		obj.QPS = 50.0
	}
	if obj.Burst == 0 {
		obj.Burst = 100
	}
}

// SetDefaults_LeaderElectionConfiguration sets defaults for the leader election of the Gardener controller manager.
func SetDefaults_LeaderElectionConfiguration(obj *LeaderElectionConfiguration) {
	if obj.ResourceLock == "" {
		obj.ResourceLock = resourcelock.LeasesResourceLock
	}

	componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(&obj.LeaderElectionConfiguration)

	if len(obj.LockObjectNamespace) == 0 {
		obj.LockObjectNamespace = ControllerManagerDefaultLockObjectNamespace
	}
	if len(obj.LockObjectName) == 0 {
		obj.LockObjectName = ControllerManagerDefaultLockObjectName
	}
}

// SetDefaults_EventControllerConfiguration sets defaults for the EventControllerConfiguration.
func SetDefaults_EventControllerConfiguration(obj *EventControllerConfiguration) {
	if obj.TTLNonShootEvents == nil {
		obj.TTLNonShootEvents = &metav1.Duration{Duration: 1 * time.Hour}
	}
}

// SetDefaults_ShootRetryControllerConfiguration sets defaults for the ShootRetryControllerConfiguration.
func SetDefaults_ShootRetryControllerConfiguration(obj *ShootRetryControllerConfiguration) {
	if obj.RetryPeriod == nil {
		obj.RetryPeriod = &metav1.Duration{Duration: 10 * time.Minute}
	}
}

// SetDefaults_ManagedSeedSetControllerConfiguration sets defaults for the given ManagedSeedSetControllerConfiguration.
func SetDefaults_ManagedSeedSetControllerConfiguration(obj *ManagedSeedSetControllerConfiguration) {
	if obj.MaxShootRetries == nil {
		v := 3
		obj.MaxShootRetries = &v
	}
}
