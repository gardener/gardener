// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/logger"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"k8s.io/apimachinery/pkg/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_SchedulerConfiguration sets defaults for the configuration of the Gardener scheduler.
func SetDefaults_SchedulerConfiguration(obj *SchedulerConfiguration) {
	if obj.LogLevel == "" {
		obj.LogLevel = logger.InfoLevel
	}

	if obj.LogFormat == "" {
		obj.LogFormat = logger.FormatJSON
	}

	if obj.Schedulers.BackupBucket == nil {
		obj.Schedulers.BackupBucket = &BackupBucketSchedulerConfiguration{
			ConcurrentSyncs: 2,
		}
	}

	if obj.Schedulers.Shoot == nil {
		obj.Schedulers.Shoot = &ShootSchedulerConfiguration{
			ConcurrentSyncs: 5,
			Strategy:        Default,
		}
	}
	if len(obj.Schedulers.Shoot.Strategy) == 0 {
		obj.Schedulers.Shoot.Strategy = Default
	}

	if obj.Schedulers.Shoot.ConcurrentSyncs == 0 {
		obj.Schedulers.Shoot.ConcurrentSyncs = 5
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
		obj.ResourceLock = resourcelock.LeasesResourceLock
	}

	componentbaseconfigv1alpha1.RecommendedDefaultLeaderElectionConfiguration(obj)

	if len(obj.ResourceNamespace) == 0 {
		obj.ResourceNamespace = SchedulerDefaultLockObjectNamespace
	}
	if len(obj.ResourceName) == 0 {
		obj.ResourceName = SchedulerDefaultLockObjectName
	}
}

// SetDefaults_ServerConfiguration sets defaults for the server configuration of the Gardener scheduler.
func SetDefaults_ServerConfiguration(obj *ServerConfiguration) {
	if obj.HealthProbes == nil {
		obj.HealthProbes = &Server{}
	}

	if len(obj.HealthProbes.BindAddress) == 0 {
		obj.HealthProbes.BindAddress = "0.0.0.0"
	}

	if obj.HealthProbes.Port == 0 {
		obj.HealthProbes.Port = 10251
	}

	if obj.Metrics == nil {
		obj.Metrics = &Server{}
	}

	if len(obj.Metrics.BindAddress) == 0 {
		obj.Metrics.BindAddress = "0.0.0.0"
	}

	if obj.Metrics.Port == 0 {
		obj.Metrics.Port = 19251
	}
}
