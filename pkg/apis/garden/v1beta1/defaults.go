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

package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_Shoot sets default values for Shoot objects.
func SetDefaults_Shoot(obj *Shoot) {
	cloud := obj.Spec.Cloud
	if cloud.AWS != nil {
		if cloud.AWS.Networks.Pods == "" {
			obj.Spec.Cloud.AWS.Networks.Pods = DefaultPodNetworkCIDR
		}
		if cloud.AWS.Networks.Services == "" {
			obj.Spec.Cloud.AWS.Networks.Services = DefaultServiceNetworkCIDR
		}
		if cloud.AWS.Networks.Nodes == "" && cloud.AWS.Networks.VPC.CIDR != "" {
			obj.Spec.Cloud.AWS.Networks.Nodes = cloud.AWS.Networks.VPC.CIDR
		}

		if obj.Spec.Backup == nil {
			obj.Spec.Backup = &Backup{
				IntervalInSecond: DefaultETCDBackupIntervalSeconds,
				Maximum:          DefaultETCDBackupMaximum,
			}
		}
	}
	if cloud.Azure != nil {
		if cloud.Azure.Networks.Pods == "" {
			obj.Spec.Cloud.Azure.Networks.Pods = DefaultPodNetworkCIDR
		}
		if cloud.Azure.Networks.Services == "" {
			obj.Spec.Cloud.Azure.Networks.Services = DefaultServiceNetworkCIDR
		}
		if cloud.Azure.Networks.Nodes == "" {
			obj.Spec.Cloud.Azure.Networks.Nodes = cloud.Azure.Networks.Workers
		}

		if obj.Spec.Backup == nil {
			obj.Spec.Backup = &Backup{
				IntervalInSecond: DefaultETCDBackupIntervalSeconds,
				Maximum:          DefaultETCDBackupMaximum,
			}
		}
	}
	if cloud.GCP != nil {
		if cloud.GCP.Networks.Pods == "" {
			obj.Spec.Cloud.GCP.Networks.Pods = DefaultPodNetworkCIDR
		}
		if cloud.GCP.Networks.Services == "" {
			obj.Spec.Cloud.GCP.Networks.Services = DefaultServiceNetworkCIDR
		}
		if cloud.GCP.Networks.Nodes == "" {
			obj.Spec.Cloud.GCP.Networks.Nodes = cloud.GCP.Networks.Workers[0]
		}
	}
	if cloud.OpenStack != nil {
		if cloud.OpenStack.Networks.Pods == "" {
			obj.Spec.Cloud.OpenStack.Networks.Pods = DefaultPodNetworkCIDR
		}
		if cloud.OpenStack.Networks.Services == "" {
			obj.Spec.Cloud.OpenStack.Networks.Services = DefaultServiceNetworkCIDR
		}
		if cloud.OpenStack.Networks.Nodes == "" {
			obj.Spec.Cloud.OpenStack.Networks.Nodes = cloud.OpenStack.Networks.Workers[0]
		}
	}

	trueVar := true
	if obj.Spec.Kubernetes.AllowPrivilegedContainers == nil {
		obj.Spec.Kubernetes.AllowPrivilegedContainers = &trueVar
	}
}
