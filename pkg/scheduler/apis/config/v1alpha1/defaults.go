// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

// SetDefaults_SchedulerConfiguration sets defaults for the configuration of the Gardener scheduler.
func SetDefaults_SchedulerConfiguration(obj *SchedulerConfiguration) {
	if obj.LogLevel == "" {
		obj.LogLevel = LogLevelInfo
	}

	if obj.LogFormat == "" {
		obj.LogFormat = LogFormatJSON
	}

	if obj.LeaderElection == nil {
		obj.LeaderElection = &componentbaseconfigv1alpha1.LeaderElectionConfiguration{}
	}
}

// SetDefaults_SchedulerControllerConfiguration sets defaults for the configuration of the controllers.
func SetDefaults_SchedulerControllerConfiguration(obj *SchedulerControllerConfiguration) {
	if obj.BackupBucket == nil {
		obj.BackupBucket = &BackupBucketSchedulerConfiguration{}
	}

	if obj.BackupBucket.ConcurrentSyncs == 0 {
		obj.BackupBucket.ConcurrentSyncs = 2
	}

	if obj.Shoot == nil {
		obj.Shoot = &ShootSchedulerConfiguration{}
	}

	if len(obj.Shoot.Strategy) == 0 {
		obj.Shoot.Strategy = Default
	}

	if obj.Shoot.ConcurrentSyncs == 0 {
		obj.Shoot.ConcurrentSyncs = 5
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

// SetDefaults_LeaderElectionConfiguration sets defaults for the leader election of the Gardener scheduler.
func SetDefaults_LeaderElectionConfiguration(obj *componentbaseconfigv1alpha1.LeaderElectionConfiguration) {
	if obj.ResourceLock == "" {
		// Don't use a constant from the client-go resourcelock package here (resourcelock is not an API package, pulls
		// in some other dependencies and is thereby not suitable to be used in this API package).
		obj.ResourceLock = "leases"
	}

	componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(obj)

	if obj.ResourceNamespace == "" {
		obj.ResourceNamespace = SchedulerDefaultLockObjectNamespace
	}
	if obj.ResourceName == "" {
		obj.ResourceName = SchedulerDefaultLockObjectName
	}
}

// SetDefaults_ServerConfiguration sets defaults for the server configuration of the Gardener scheduler.
func SetDefaults_ServerConfiguration(obj *ServerConfiguration) {
	if obj.HealthProbes == nil {
		obj.HealthProbes = &Server{}
	}

	if obj.HealthProbes.Port == 0 {
		obj.HealthProbes.Port = 10251
	}

	if obj.Metrics == nil {
		obj.Metrics = &Server{}
	}

	if obj.Metrics.Port == 0 {
		obj.Metrics.Port = 19251
	}
}
