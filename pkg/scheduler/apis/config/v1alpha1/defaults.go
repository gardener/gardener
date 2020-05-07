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
	"time"

	"k8s.io/client-go/tools/leaderelection/resourcelock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_SchedulerConfiguration sets defaults for the configuration of the Gardener scheduler.
func SetDefaults_SchedulerConfiguration(obj *SchedulerConfiguration) {
	if len(obj.Server.HTTP.BindAddress) == 0 {
		obj.Server.HTTP.BindAddress = "0.0.0.0"
	}
	if obj.Server.HTTP.Port == 0 {
		obj.Server.HTTP.Port = 10251
	}

	if obj.Schedulers.BackupBucket == nil {
		obj.Schedulers.BackupBucket = &BackupBucketSchedulerConfiguration{
			ConcurrentSyncs: 2,
			RetrySyncPeriod: metav1.Duration{
				Duration: 15 * time.Second,
			},
		}
	}

	if obj.Schedulers.Shoot == nil {
		obj.Schedulers.Shoot = &ShootSchedulerConfiguration{
			ConcurrentSyncs: 5,
			RetrySyncPeriod: metav1.Duration{
				Duration: 15 * time.Second,
			},
			Strategy: Default,
		}
	}
	if len(obj.Schedulers.Shoot.Strategy) == 0 {
		obj.Schedulers.Shoot.Strategy = Default
	}

	if obj.Schedulers.Shoot.ConcurrentSyncs == 0 {
		obj.Schedulers.Shoot.ConcurrentSyncs = 5
	}

}

// SetDefaults_ClientConnection sets defaults for the client connection.
func SetDefaults_ClientConnection(obj *componentbaseconfigv1alpha1.ClientConnectionConfiguration) {
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
		obj.LockObjectNamespace = SchedulerDefaultLockObjectNamespace
	}
	if len(obj.LockObjectName) == 0 {
		obj.LockObjectName = SchedulerDefaultLockObjectName
	}
}
