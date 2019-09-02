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
	"math"

	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/apis/garden"
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
		cloud            = obj.Spec.Cloud
		defaultProxyMode = ProxyModeIPTables
	)

	if obj.Spec.Networking == nil {
		obj.Spec.Networking = &Networking{}
	}
	if len(obj.Spec.Networking.Type) == 0 {
		obj.Spec.Networking.Type = CalicoNetworkType
	}

	if cloud.AWS != nil {
		if cloud.AWS.Networks.Nodes == nil {
			if cloud.AWS.Networks.VPC.CIDR != nil {
				obj.Spec.Cloud.AWS.Networks.Nodes = cloud.AWS.Networks.VPC.CIDR
				if obj.Spec.Networking.Nodes == nil {
					obj.Spec.Networking.Nodes = cloud.AWS.Networks.VPC.CIDR
				}
			} else if len(cloud.AWS.Networks.Workers) == 1 {
				obj.Spec.Cloud.AWS.Networks.Nodes = &cloud.AWS.Networks.Workers[0]
				if obj.Spec.Networking.Nodes == nil {
					obj.Spec.Networking.Nodes = &cloud.AWS.Networks.Workers[0]
				}
			}
		}
		if obj.Spec.Kubernetes.KubeControllerManager == nil || obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize == nil {
			SetNodeCIDRMaskSize(&obj.Spec.Kubernetes, CalculateDefaultNodeCIDRMaskSize(&obj.Spec.Kubernetes, getShootCloudProviderWorkers(CloudProviderAWS, obj)))
		}
	}

	if cloud.Azure != nil {
		if cloud.Azure.Networks.Nodes == nil {
			obj.Spec.Cloud.Azure.Networks.Nodes = &cloud.Azure.Networks.Workers
			if obj.Spec.Networking.Nodes == nil {
				obj.Spec.Networking.Nodes = &cloud.Azure.Networks.Workers
			}
		}
		if obj.Spec.Kubernetes.KubeControllerManager == nil || obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize == nil {
			SetNodeCIDRMaskSize(&obj.Spec.Kubernetes, CalculateDefaultNodeCIDRMaskSize(&obj.Spec.Kubernetes, getShootCloudProviderWorkers(CloudProviderAzure, obj)))
		}
	}

	if cloud.GCP != nil {
		if cloud.GCP.Networks.Nodes == nil && len(cloud.GCP.Networks.Workers) == 1 {
			obj.Spec.Cloud.GCP.Networks.Nodes = &cloud.GCP.Networks.Workers[0]
			if obj.Spec.Networking.Nodes == nil {
				obj.Spec.Networking.Nodes = &cloud.GCP.Networks.Workers[0]
			}
		}
		if obj.Spec.Kubernetes.KubeControllerManager == nil || obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize == nil {
			SetNodeCIDRMaskSize(&obj.Spec.Kubernetes, CalculateDefaultNodeCIDRMaskSize(&obj.Spec.Kubernetes, getShootCloudProviderWorkers(CloudProviderGCP, obj)))
		}
	}

	if cloud.Alicloud != nil {
		if cloud.Alicloud.Networks.Nodes == nil {
			if cloud.Alicloud.Networks.VPC.CIDR != nil {
				obj.Spec.Cloud.Alicloud.Networks.Nodes = cloud.Alicloud.Networks.VPC.CIDR
				if obj.Spec.Networking.Nodes == nil {
					obj.Spec.Networking.Nodes = cloud.Alicloud.Networks.VPC.CIDR
				}
			} else if len(cloud.Alicloud.Networks.Workers) == 1 {
				obj.Spec.Cloud.Alicloud.Networks.Nodes = &cloud.Alicloud.Networks.Workers[0]
				if obj.Spec.Networking.Nodes == nil {
					obj.Spec.Networking.Nodes = &cloud.Alicloud.Networks.Workers[0]
				}
			}
		}
		if obj.Spec.Kubernetes.KubeControllerManager == nil || obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize == nil {
			SetNodeCIDRMaskSize(&obj.Spec.Kubernetes, CalculateDefaultNodeCIDRMaskSize(&obj.Spec.Kubernetes, getShootCloudProviderWorkers(CloudProviderAlicloud, obj)))
		}
	}

	if cloud.OpenStack != nil {
		if cloud.OpenStack.Networks.Nodes == nil && len(cloud.OpenStack.Networks.Workers) == 1 {
			obj.Spec.Cloud.OpenStack.Networks.Nodes = &cloud.OpenStack.Networks.Workers[0]
			if obj.Spec.Networking.Nodes == nil {
				obj.Spec.Networking.Nodes = &cloud.OpenStack.Networks.Workers[0]
			}
		}
		if obj.Spec.Kubernetes.KubeControllerManager == nil || obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize == nil {
			SetNodeCIDRMaskSize(&obj.Spec.Kubernetes, CalculateDefaultNodeCIDRMaskSize(&obj.Spec.Kubernetes, getShootCloudProviderWorkers(CloudProviderOpenStack, obj)))
		}
	}

	if cloud.Packet != nil {
		if obj.Spec.Kubernetes.KubeControllerManager == nil || obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize == nil {
			SetNodeCIDRMaskSize(&obj.Spec.Kubernetes, CalculateDefaultNodeCIDRMaskSize(&obj.Spec.Kubernetes, getShootCloudProviderWorkers(CloudProviderPacket, obj)))
		}
	}

	trueVar := true
	if obj.Spec.Kubernetes.AllowPrivilegedContainers == nil {
		obj.Spec.Kubernetes.AllowPrivilegedContainers = &trueVar
	}

	if obj.Spec.Kubernetes.KubeAPIServer != nil {
		if obj.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication == nil {
			obj.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = &trueVar
		}
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
				KubernetesVersion:   true,
				MachineImageVersion: &trueVar,
			},
			TimeWindow: &MaintenanceTimeWindow{
				Begin: mt.Begin().Formatted(),
				End:   mt.End().Formatted(),
			},
		}
	} else {
		if obj.Spec.Maintenance.AutoUpdate == nil {
			obj.Spec.Maintenance.AutoUpdate = &MaintenanceAutoUpdate{
				KubernetesVersion:   trueVar,
				MachineImageVersion: &trueVar,
			}
		}

		if obj.Spec.Maintenance.AutoUpdate.MachineImageVersion == nil {
			obj.Spec.Maintenance.AutoUpdate.MachineImageVersion = &trueVar
		}

		if obj.Spec.Maintenance.TimeWindow == nil {
			mt := utils.RandomMaintenanceTimeWindow()

			obj.Spec.Maintenance.TimeWindow = &MaintenanceTimeWindow{
				Begin: mt.Begin().Formatted(),
				End:   mt.End().Formatted(),
			}
		}
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

	var (
		defaultPodCIDR             = DefaultPodNetworkCIDR
		defaultServiceCIDR         = DefaultServiceNetworkCIDR
		defaultPodCIDRAlicloud     = DefaultPodNetworkCIDRAlicloud
		defaultServiceCIDRAlicloud = DefaultServiceNetworkCIDRAlicloud
	)

	if obj.Spec.Networks.ShootDefaults == nil {
		obj.Spec.Networks.ShootDefaults = &ShootNetworks{}
	}

	if v, ok := obj.Annotations[garden.MigrationSeedProviderType]; ok && v == "alicloud" {
		if obj.Spec.Networks.ShootDefaults.Pods == nil && !gardencorev1alpha1helper.NetworksIntersect(obj.Spec.Networks.Pods, defaultPodCIDRAlicloud) {
			obj.Spec.Networks.ShootDefaults.Pods = &defaultPodCIDRAlicloud
		}
		if obj.Spec.Networks.ShootDefaults.Services == nil && !gardencorev1alpha1helper.NetworksIntersect(obj.Spec.Networks.Services, defaultServiceCIDRAlicloud) {
			obj.Spec.Networks.ShootDefaults.Services = &defaultServiceCIDRAlicloud
		}
	} else {
		if obj.Spec.Networks.ShootDefaults.Pods == nil && !gardencorev1alpha1helper.NetworksIntersect(obj.Spec.Networks.Pods, defaultPodCIDR) {
			obj.Spec.Networks.ShootDefaults.Pods = &defaultPodCIDR
		}
		if obj.Spec.Networks.ShootDefaults.Services == nil && !gardencorev1alpha1helper.NetworksIntersect(obj.Spec.Networks.Services, defaultServiceCIDR) {
			obj.Spec.Networks.ShootDefaults.Services = &defaultServiceCIDR
		}
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

// SetDefaults_KubernetesDashboard sets default values for KubernetesDashboard objects.
func SetDefaults_KubernetesDashboard(obj *KubernetesDashboard) {
	defaultAuthMode := KubernetesDashboardAuthModeBasic
	if obj.AuthenticationMode == nil {
		obj.AuthenticationMode = &defaultAuthMode
	}
}

// SetDefaults_Worker sets default values for Worker objects.
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

// CalculateDefaultNodeCIDRMaskSize calculates a default NodeCIDRMaskSize CIDR from the highest maxPod setting in the shoot
func CalculateDefaultNodeCIDRMaskSize(kubernetes *Kubernetes, workers []Worker) *int {
	var maxPod int32
	if kubernetes.Kubelet != nil && kubernetes.Kubelet.MaxPods != nil {
		maxPod = *kubernetes.Kubelet.MaxPods
	}

	for _, worker := range workers {
		if worker.Kubelet != nil && worker.Kubelet.MaxPods != nil && *worker.Kubelet.MaxPods > maxPod {
			maxPod = *worker.Kubelet.MaxPods
		}
	}

	if maxPod == 0 {
		// default maxPod setting on kubelet
		maxPod = 110
	}

	// by having approximately twice as many available IP addresses as possible Pods, Kubernetes is able to mitigate IP address reuse as Pods are added to and removed from a node.
	nodeCidrRange := 32 - int(math.Ceil(math.Log2(float64(maxPod*2))))
	return &nodeCidrRange
}

// SetNodeCIDRMaskSize sets the NodeCIDRMaskSize on the shoot
func SetNodeCIDRMaskSize(kubernetes *Kubernetes, requiredNodeCIDRMaskSize *int) {
	if kubernetes.KubeControllerManager == nil {
		kubernetes.KubeControllerManager = &KubeControllerManagerConfig{NodeCIDRMaskSize: requiredNodeCIDRMaskSize}
	} else {
		kubernetes.KubeControllerManager.NodeCIDRMaskSize = requiredNodeCIDRMaskSize
	}
}

// getShootCloudProviderWorkers retrieves the cloud-specific workers of the given Shoot.
func getShootCloudProviderWorkers(cloudProvider CloudProvider, shoot *Shoot) []Worker {
	var (
		cloud   = shoot.Spec.Cloud
		workers []Worker
	)

	switch cloudProvider {
	case CloudProviderAWS:
		for _, worker := range cloud.AWS.Workers {
			workers = append(workers, worker.Worker)
		}
	case CloudProviderAzure:
		for _, worker := range cloud.Azure.Workers {
			workers = append(workers, worker.Worker)
		}
	case CloudProviderGCP:
		for _, worker := range cloud.GCP.Workers {
			workers = append(workers, worker.Worker)
		}
	case CloudProviderAlicloud:
		for _, worker := range cloud.Alicloud.Workers {
			workers = append(workers, worker.Worker)
		}
	case CloudProviderOpenStack:
		for _, worker := range cloud.OpenStack.Workers {
			workers = append(workers, worker.Worker)
		}
	case CloudProviderPacket:
		for _, worker := range cloud.Packet.Workers {
			workers = append(workers, worker.Worker)
		}
	}

	return workers
}
