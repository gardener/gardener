// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
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

	if obj.Controllers.CloudProfile == nil {
		obj.Controllers.CloudProfile = &CloudProfileControllerConfiguration{
			ConcurrentSyncs: 5,
		}
	}

	if obj.Controllers.ControllerRegistration == nil {
		obj.Controllers.ControllerRegistration = &ControllerRegistrationControllerConfiguration{
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
}

// SetDefaults_GardenClientConnection sets defaults for the client connection.
func SetDefaults_GardenClientConnection(obj *componentbaseconfigv1alpha1.ClientConnectionConfiguration) {
	//componentbaseconfigv1alpha1.RecommendedDefaultClientConnectionConfiguration(obj)
	// https://github.com/kubernetes/client-go/issues/76#issuecomment-396170694
	if len(obj.AcceptContentTypes) == 0 {
		obj.AcceptContentTypes = "application/json"
	}
	if len(obj.ContentType) == 0 {
		obj.ContentType = "application/json"
	}
	if obj.QPS == 0.0 {
		obj.QPS = 50.0
	}
	if obj.Burst == 0 {
		obj.Burst = 100
	}
}

// SetDefaults_LeaderElectionConfiguration sets defaults for the leader election of the Gardener controller manager.
func SetDefaults_LeaderElectionConfiguration(obj *LeaderElectionConfiguration) {
	componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(&obj.LeaderElectionConfiguration)

	obj.ResourceLock = resourcelock.ConfigMapsResourceLock

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
