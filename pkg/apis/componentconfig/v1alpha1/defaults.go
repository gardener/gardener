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
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_ControllerManagerConfiguration sets defaults for the configuration of the Gardener controller manager.
func SetDefaults_ControllerManagerConfiguration(obj *ControllerManagerConfiguration) {
	if len(obj.Server.BindAddress) == 0 {
		obj.Server.BindAddress = "0.0.0.0"
	}
	if obj.Server.Port == 0 {
		obj.Server.Port = 2718
	}

	if len(obj.ClientConnection.ContentType) == 0 {
		obj.ClientConnection.ContentType = "application/vnd.kubernetes.protobuf"
	}
	if obj.ClientConnection.QPS == 0.0 {
		obj.ClientConnection.QPS = 50.0
	}
	if obj.ClientConnection.Burst == 0 {
		obj.ClientConnection.Burst = 100
	}

	if obj.GardenerClientConnection == nil {
		obj.GardenerClientConnection = &obj.ClientConnection
	} else {
		if len(obj.GardenerClientConnection.KubeConfigFile) == 0 {
			obj.GardenerClientConnection.KubeConfigFile = obj.ClientConnection.KubeConfigFile
		}
		if len(obj.GardenerClientConnection.AcceptContentTypes) == 0 {
			obj.GardenerClientConnection.AcceptContentTypes = "application/json"
		}
		if len(obj.GardenerClientConnection.ContentType) == 0 {
			obj.GardenerClientConnection.ContentType = "application/json"
		}
		if obj.GardenerClientConnection.QPS == 0.0 {
			obj.GardenerClientConnection.QPS = obj.ClientConnection.QPS
		}
		if obj.GardenerClientConnection.Burst == 0 {
			obj.GardenerClientConnection.Burst = obj.ClientConnection.Burst
		}
	}

	if len(obj.LeaderElection.LockObjectNamespace) == 0 {
		obj.LeaderElection.LockObjectNamespace = ControllerManagerDefaultLockObjectNamespace
	}
	if len(obj.LeaderElection.LockObjectName) == 0 {
		obj.LeaderElection.LockObjectName = ControllerManagerDefaultLockObjectName
	}

	if obj.Controllers.CloudProfile == nil {
		obj.Controllers.CloudProfile = &CloudProfileControllerConfiguration{
			ConcurrentSyncs: 5,
		}
	}
	if obj.Controllers.SecretBinding == nil {
		obj.Controllers.SecretBinding = &SecretBindingControllerConfiguration{
			ConcurrentSyncs: 5,
		}
	}
	if obj.Controllers.Quota == nil {
		obj.Controllers.Quota = &QuotaControllerConfiguration{
			ConcurrentSyncs: 5,
		}
	}
	if obj.Controllers.Seed == nil {
		obj.Controllers.Seed = &SeedControllerConfiguration{
			ConcurrentSyncs: 5,
		}
	}

	if obj.Controllers.Shoot.RespectSyncPeriodOverwrite == nil {
		falseVar := false
		obj.Controllers.Shoot.RespectSyncPeriodOverwrite = &falseVar
	}
	if obj.Controllers.Shoot.RetrySyncPeriod == nil {
		durationVar := metav1.Duration{Duration: 15 * time.Second}
		obj.Controllers.Shoot.RetrySyncPeriod = &durationVar
	}

	if obj.Controllers.BackupInfrastructure.DeletionGracePeriodDays == nil || *obj.Controllers.BackupInfrastructure.DeletionGracePeriodDays < 0 {
		var defaultBackupInfrastructureDeletionGracePeriodDays = DefaultBackupInfrastructureDeletionGracePeriodDays
		obj.Controllers.BackupInfrastructure.DeletionGracePeriodDays = &defaultBackupInfrastructureDeletionGracePeriodDays
	}
}

// SetDefaults_LeaderElectionConfiguration sets defaults for the leader election of the Gardener controller manager.
func SetDefaults_LeaderElectionConfiguration(obj *LeaderElectionConfiguration) {
	zero := metav1.Duration{}
	if obj.LeaseDuration == zero {
		obj.LeaseDuration = metav1.Duration{Duration: 15 * time.Second}
	}
	if obj.RenewDeadline == zero {
		obj.RenewDeadline = metav1.Duration{Duration: 10 * time.Second}
	}
	if obj.RetryPeriod == zero {
		obj.RetryPeriod = metav1.Duration{Duration: 2 * time.Second}
	}
	if obj.ResourceLock == "" {
		obj.ResourceLock = resourcelock.ConfigMapsResourceLock
	}
}
