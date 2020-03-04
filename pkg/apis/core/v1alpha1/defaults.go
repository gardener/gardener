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
	"math"

	"github.com/gardener/gardener/pkg/utils"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
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

// SetDefaults_Project sets default values for Project objects.
func SetDefaults_Project(obj *Project) {
	defaultSubject(obj.Spec.Owner)

	for i, member := range obj.Spec.Members {
		defaultSubject(&obj.Spec.Members[i].Subject)

		if len(member.Role) == 0 && len(member.Roles) == 0 {
			obj.Spec.Members[i].Role = ProjectMemberViewer
		}
	}
}

func defaultSubject(obj *rbacv1.Subject) {
	if obj != nil && len(obj.APIGroup) == 0 {
		switch obj.Kind {
		case rbacv1.ServiceAccountKind:
			obj.APIGroup = ""
		case rbacv1.UserKind:
			obj.APIGroup = rbacv1.GroupName
		case rbacv1.GroupKind:
			obj.APIGroup = rbacv1.GroupName
		}
	}
}

// SetDefaults_MachineType sets default values for MachineType objects.
func SetDefaults_MachineType(obj *MachineType) {
	if obj.Usable == nil {
		trueVar := true
		obj.Usable = &trueVar
	}
}

// SetDefaults_VolumeType sets default values for VolumeType objects.
func SetDefaults_VolumeType(obj *VolumeType) {
	if obj.Usable == nil {
		trueVar := true
		obj.Usable = &trueVar
	}
}

// SetDefaults_Shoot sets default values for Shoot objects.
func SetDefaults_Shoot(obj *Shoot) {
	k8sVersionLessThan116, _ := versionutils.CompareVersions(obj.Spec.Kubernetes.Version, "<", "1.16")
	// Error is ignored here because we cannot do anything meaningful with it.
	// k8sVersionLessThan116 will default to `false`.

	trueVar := true
	falseVar := false

	if obj.Spec.Kubernetes.AllowPrivilegedContainers == nil {
		obj.Spec.Kubernetes.AllowPrivilegedContainers = &trueVar
	}

	if obj.Spec.Kubernetes.KubeAPIServer == nil {
		obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{}
	}
	if obj.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication == nil {
		if k8sVersionLessThan116 {
			obj.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = &trueVar
		} else {
			obj.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = &falseVar
		}
	}

	if obj.Spec.Kubernetes.KubeControllerManager == nil {
		obj.Spec.Kubernetes.KubeControllerManager = &KubeControllerManagerConfig{}
	}
	if obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize == nil {
		obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = calculateDefaultNodeCIDRMaskSize(obj.Spec.Kubernetes.Kubelet, obj.Spec.Provider.Workers)
	}

	if obj.Spec.Kubernetes.KubeProxy == nil {
		obj.Spec.Kubernetes.KubeProxy = &KubeProxyConfig{}
	}
	if obj.Spec.Kubernetes.KubeProxy.Mode == nil {
		defaultProxyMode := ProxyModeIPTables
		obj.Spec.Kubernetes.KubeProxy.Mode = &defaultProxyMode
	}

	if obj.Spec.Addons == nil {
		obj.Spec.Addons = &Addons{}
	}
	if obj.Spec.Addons.KubernetesDashboard == nil {
		obj.Spec.Addons.KubernetesDashboard = &KubernetesDashboard{}
	}
	if obj.Spec.Addons.KubernetesDashboard.AuthenticationMode == nil {
		var defaultAuthMode string
		if *obj.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication {
			defaultAuthMode = KubernetesDashboardAuthModeBasic
		} else {
			defaultAuthMode = KubernetesDashboardAuthModeToken
		}
		obj.Spec.Addons.KubernetesDashboard.AuthenticationMode = &defaultAuthMode
	}

	if obj.Spec.Purpose == nil {
		p := ShootPurposeEvaluation
		obj.Spec.Purpose = &p
	}
}

// SetDefaults_Maintenance sets default values for Maintenance objects.
func SetDefaults_Maintenance(obj *Maintenance) {
	if obj == nil {
		obj = &Maintenance{}
	}

	if obj.AutoUpdate == nil {
		obj.AutoUpdate = &MaintenanceAutoUpdate{
			KubernetesVersion:   true,
			MachineImageVersion: true,
		}
	}

	if obj.TimeWindow == nil {
		mt := utils.RandomMaintenanceTimeWindow()
		obj.TimeWindow = &MaintenanceTimeWindow{
			Begin: mt.Begin().Formatted(),
			End:   mt.End().Formatted(),
		}
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

// SetDefaults_NginxIngress sets default values for NginxIngress objects.
func SetDefaults_NginxIngress(obj *NginxIngress) {
	if obj.ExternalTrafficPolicy == nil {
		v := corev1.ServiceExternalTrafficPolicyTypeCluster
		obj.ExternalTrafficPolicy = &v
	}
}

// Helper functions

func calculateDefaultNodeCIDRMaskSize(kubelet *KubeletConfig, workers []Worker) *int32 {
	var maxPods int32 = 110 // default maxPods setting on kubelet

	if kubelet != nil && kubelet.MaxPods != nil {
		maxPods = *kubelet.MaxPods
	}

	for _, worker := range workers {
		if worker.Kubernetes != nil && worker.Kubernetes.Kubelet != nil && worker.Kubernetes.Kubelet.MaxPods != nil && *worker.Kubernetes.Kubelet.MaxPods > maxPods {
			maxPods = *worker.Kubernetes.Kubelet.MaxPods
		}
	}

	// by having approximately twice as many available IP addresses as possible Pods, Kubernetes is able to mitigate IP address reuse as Pods are added to and removed from a node.
	nodeCidrRange := int32(32 - int(math.Ceil(math.Log2(float64(maxPods*2)))))
	return &nodeCidrRange
}
