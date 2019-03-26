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

package v1beta1

import (
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_Shoot sets default values for Shoot objects.
func SetDefaults_Shoot(obj *Shoot) {
	var (
		cloud              = obj.Spec.Cloud
		defaultPodCIDR     = gardencorev1alpha1.DefaultPodNetworkCIDR
		defaultServiceCIDR = gardencorev1alpha1.DefaultServiceNetworkCIDR
		defaultProxyMode   = ProxyModeIPTables
	)

	if cloud.AWS != nil {
		if cloud.AWS.Networks.Pods == nil {
			obj.Spec.Cloud.AWS.Networks.Pods = &defaultPodCIDR
		}
		if cloud.AWS.Networks.Services == nil {
			obj.Spec.Cloud.AWS.Networks.Services = &defaultServiceCIDR
		}
		if cloud.AWS.Networks.Nodes == nil {
			if cloud.AWS.Networks.VPC.CIDR != nil {
				obj.Spec.Cloud.AWS.Networks.Nodes = cloud.AWS.Networks.VPC.CIDR
			} else if len(cloud.AWS.Networks.Workers) > 0 {
				obj.Spec.Cloud.AWS.Networks.Nodes = &cloud.AWS.Networks.Workers[0]
			}
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
		if cloud.GCP.Networks.Nodes == nil && len(cloud.GCP.Networks.Workers) > 0 {
			obj.Spec.Cloud.GCP.Networks.Nodes = &cloud.GCP.Networks.Workers[0]
		}
	}

	if cloud.Alicloud != nil {
		if cloud.Alicloud.Networks.Pods == nil {
			podCIDR := gardencorev1alpha1.CIDR("100.64.0.0/11")
			obj.Spec.Cloud.Alicloud.Networks.Pods = &podCIDR
		}
		if cloud.Alicloud.Networks.Services == nil {
			svcCIDR := gardencorev1alpha1.CIDR("100.104.0.0/13")
			obj.Spec.Cloud.Alicloud.Networks.Services = &svcCIDR
		}
		if cloud.Alicloud.Networks.Nodes == nil {
			if cloud.Alicloud.Networks.VPC.CIDR != nil {
				obj.Spec.Cloud.Alicloud.Networks.Nodes = cloud.Alicloud.Networks.VPC.CIDR
			} else if len(cloud.Alicloud.Networks.Workers) > 0 {
				obj.Spec.Cloud.Alicloud.Networks.Nodes = &cloud.Alicloud.Networks.Workers[0]
			}
		}
	}

	if cloud.OpenStack != nil {
		if cloud.OpenStack.Networks.Pods == nil {
			obj.Spec.Cloud.OpenStack.Networks.Pods = &defaultPodCIDR
		}
		if cloud.OpenStack.Networks.Services == nil {
			obj.Spec.Cloud.OpenStack.Networks.Services = &defaultServiceCIDR
		}
		if cloud.OpenStack.Networks.Nodes == nil && len(cloud.OpenStack.Networks.Workers) > 0 {
			obj.Spec.Cloud.OpenStack.Networks.Nodes = &cloud.OpenStack.Networks.Workers[0]
		}
	}

	if cloud.Packet != nil {
		if cloud.Packet.Networks.Pods == nil {
			obj.Spec.Cloud.Packet.Networks.Pods = &defaultPodCIDR
		}
		if cloud.Packet.Networks.Services == nil {
			obj.Spec.Cloud.Packet.Networks.Services = &defaultServiceCIDR
		}
	}

	if cloud.Local != nil {
		if cloud.Local.Networks.Pods == nil {
			obj.Spec.Cloud.Local.Networks.Pods = &defaultPodCIDR
		}
		if cloud.Local.Networks.Services == nil {
			obj.Spec.Cloud.Local.Networks.Services = &defaultServiceCIDR
		}
		if cloud.Local.Networks.Nodes == nil && len(cloud.Local.Networks.Workers) > 0 {
			obj.Spec.Cloud.Local.Networks.Nodes = &cloud.Local.Networks.Workers[0]
		}
	}

	trueVar := true
	if obj.Spec.Kubernetes.AllowPrivilegedContainers == nil {
		obj.Spec.Kubernetes.AllowPrivilegedContainers = &trueVar
	}

	if obj.Spec.Kubernetes.KubeProxy != nil {
		if obj.Spec.Kubernetes.KubeProxy.Mode == nil {
			obj.Spec.Kubernetes.KubeProxy.Mode = &defaultProxyMode
		}
	}

	if obj.Spec.Maintenance == nil {
		mt := utils.RandomMaintenanceTimeWindow()

		obj.Spec.Maintenance = &Maintenance{
			AutoUpdate: &MaintenanceAutoUpdate{
				KubernetesVersion: trueVar,
			},
			TimeWindow: &MaintenanceTimeWindow{
				Begin: mt.Begin().Formatted(),
				End:   mt.End().Formatted(),
			},
		}
	} else {
		if obj.Spec.Maintenance.AutoUpdate == nil {
			obj.Spec.Maintenance.AutoUpdate = &MaintenanceAutoUpdate{
				KubernetesVersion: trueVar,
			}
		}

		if obj.Spec.Maintenance.TimeWindow == nil {
			mt := utils.RandomMaintenanceTimeWindow()

			obj.Spec.Maintenance.TimeWindow = &MaintenanceTimeWindow{
				Begin: mt.Begin().Formatted(),
				End:   mt.End().Formatted(),
			}
		}
	}

	if obj.Spec.DNS.Provider == DNSUnmanaged && obj.Spec.DNS.Domain == nil {
		defaultDomain := DefaultDomain
		obj.Spec.DNS.Domain = &defaultDomain
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

// SetDefaults_Project sets default values for Project objects.
func SetDefaults_Project(obj *Project) {
	if obj.Spec.Owner != nil && len(obj.Spec.Owner.APIGroup) == 0 {
		switch obj.Spec.Owner.Kind {
		case rbacv1.ServiceAccountKind:
			obj.Spec.Owner.APIGroup = ""
		case rbacv1.UserKind:
			obj.Spec.Owner.APIGroup = rbacv1.GroupName
		case rbacv1.GroupKind:
			obj.Spec.Owner.APIGroup = rbacv1.GroupName
		}
	}
}

func SetDefaults_Worker(obj *Worker) {
	if obj.MaxSurge == nil {
		obj.MaxSurge = &DefaultWorkerMaxSurge
	}
	if obj.MaxUnavailable == nil {
		obj.MaxUnavailable = &DefaultWorkerMaxUnavailable
	}
}

// SetDefaults_SecretBinding sets default values for SecretBinding objects.
func SetDefaults_SecretBinding(obj *SecretBinding) {
	if len(obj.SecretRef.Namespace) == 0 {
		obj.SecretRef.Namespace = obj.Namespace
	}

	for i, quota := range obj.Quotas {
		if len(quota.Namespace) == 0 {
			obj.Quotas[i].Namespace = obj.Namespace
		}
	}
}

// SetDefaults_MachineType sets default values for MachineType objects.
func SetDefaults_MachineType(obj *MachineType) {
	trueVar := true
	if obj.Usable == nil {
		obj.Usable = &trueVar
	}
}

// SetDefaults_VolumeType sets default values for VolumeType objects.
func SetDefaults_VolumeType(obj *VolumeType) {
	trueVar := true
	if obj.Usable == nil {
		obj.Usable = &trueVar
	}
}
