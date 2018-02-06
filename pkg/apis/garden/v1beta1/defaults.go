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
	if obj.Spec.Backup == nil {
		obj.Spec.Backup = &Backup{
			IntervalInSecond: DefaultETCDBackupIntervalSeconds,
			Maximum:          DefaultETCDBackupMaximum,
		}
	}

	var (
		cloud              = obj.Spec.Cloud
		defaultPodCIDR     = DefaultPodNetworkCIDR
		defaultServiceCIDR = DefaultServiceNetworkCIDR
	)

	if cloud.AWS != nil {
		if cloud.AWS.Networks.Pods == nil {
			obj.Spec.Cloud.AWS.Networks.Pods = &defaultPodCIDR
		}
		if cloud.AWS.Networks.Services == nil {
			obj.Spec.Cloud.AWS.Networks.Services = &defaultServiceCIDR
		}
		if cloud.AWS.Networks.Nodes == nil && cloud.AWS.Networks.VPC.CIDR != nil {
			obj.Spec.Cloud.AWS.Networks.Nodes = cloud.AWS.Networks.VPC.CIDR
		}
	}

	if cloud.Azure != nil {
		if cloud.Azure.Networks.Pods == nil {
			obj.Spec.Cloud.Azure.Networks.Pods = &defaultPodCIDR
		}
		if cloud.Azure.Networks.Services == nil {
			obj.Spec.Cloud.Azure.Networks.Services = &defaultServiceCIDR
		}
		if cloud.Azure.Networks.Nodes == nil {
			obj.Spec.Cloud.Azure.Networks.Nodes = &cloud.Azure.Networks.Workers
		}
	}

	if cloud.GCP != nil {
		if cloud.GCP.Networks.Pods == nil {
			obj.Spec.Cloud.GCP.Networks.Pods = &defaultPodCIDR
		}
		if cloud.GCP.Networks.Services == nil {
			obj.Spec.Cloud.GCP.Networks.Services = &defaultServiceCIDR
		}
		if cloud.GCP.Networks.Nodes == nil {
			obj.Spec.Cloud.GCP.Networks.Nodes = &cloud.GCP.Networks.Workers[0]
		}
	}

	if cloud.OpenStack != nil {
		if cloud.OpenStack.Networks.Pods == nil {
			obj.Spec.Cloud.OpenStack.Networks.Pods = &defaultPodCIDR
		}
		if cloud.OpenStack.Networks.Services == nil {
			obj.Spec.Cloud.OpenStack.Networks.Services = &defaultServiceCIDR
		}
		if cloud.OpenStack.Networks.Nodes == nil {
			obj.Spec.Cloud.OpenStack.Networks.Nodes = &cloud.OpenStack.Networks.Workers[0]
		}
	}

	trueVar := true
	if obj.Spec.Kubernetes.AllowPrivilegedContainers == nil {
		obj.Spec.Kubernetes.AllowPrivilegedContainers = &trueVar
	}
}

// SetDefaults_Seed sets default values for Seed objects.
func SetDefaults_Seed(obj *Seed) {
	trueVar := true
	if obj.Spec.Visible == nil {
		obj.Spec.Visible = &trueVar
	}
	falseVar := false
	if obj.Spec.Protected == nil {
		obj.Spec.Protected = &falseVar
	}
}
