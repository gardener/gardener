// Copyright 2018 The Gardener Authors.
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

// SetDefaults_ControllerManagerConfiguration sets defaults for the configuration of the Garden controller manager.
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

	if len(obj.LeaderElection.LockObjectNamespace) == 0 {
		obj.LeaderElection.LockObjectNamespace = ControllerManagerDefaultLockObjectNamespace
	}
	if len(obj.LeaderElection.LockObjectName) == 0 {
		obj.LeaderElection.LockObjectName = ControllerManagerDefaultLockObjectName
	}
}

// SetDefaults_LeaderElectionConfiguration sets defaults for the leader election of the Garden controller manager.
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
