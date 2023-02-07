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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/timewindow"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
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

// SetDefaults_Seed sets default values for Seed objects.
func SetDefaults_Seed(obj *Seed) {
	if obj.Spec.Settings == nil {
		obj.Spec.Settings = &SeedSettings{}
	}

	if obj.Spec.Settings.ExcessCapacityReservation == nil {
		obj.Spec.Settings.ExcessCapacityReservation = &SeedSettingExcessCapacityReservation{Enabled: true}
	}

	if obj.Spec.Settings.Scheduling == nil {
		obj.Spec.Settings.Scheduling = &SeedSettingScheduling{Visible: true}
	}

	if obj.Spec.Settings.VerticalPodAutoscaler == nil {
		obj.Spec.Settings.VerticalPodAutoscaler = &SeedSettingVerticalPodAutoscaler{Enabled: true}
	}

	if obj.Spec.Settings.OwnerChecks == nil {
		obj.Spec.Settings.OwnerChecks = &SeedSettingOwnerChecks{Enabled: true}
	}

	if obj.Spec.Settings.DependencyWatchdog == nil {
		obj.Spec.Settings.DependencyWatchdog = &SeedSettingDependencyWatchdog{}
	}
}

// SetDefaults_SeedNetworks sets default values for SeedNetworks objects.
func SetDefaults_SeedNetworks(obj *SeedNetworks) {
	if len(obj.IPFamilies) == 0 {
		obj.IPFamilies = []IPFamily{IPFamilyIPv4}
	}
}

// SetDefaults_SeedSettingDependencyWatchdog sets defaults for SeedSettingDependencyWatchdog objects.
func SetDefaults_SeedSettingDependencyWatchdog(obj *SeedSettingDependencyWatchdog) {
	if obj.Endpoint == nil {
		obj.Endpoint = &SeedSettingDependencyWatchdogEndpoint{Enabled: true}
	}
	if obj.Probe == nil {
		obj.Probe = &SeedSettingDependencyWatchdogProbe{Enabled: true}
	}
}

// SetDefaults_Shoot sets default values for Shoot objects.
func SetDefaults_Shoot(obj *Shoot) {
	// Errors are ignored here because we cannot do anything meaningful with them - variables will default to `false`.
	k8sLess125, _ := versionutils.CheckVersionMeetsConstraint(obj.Spec.Kubernetes.Version, "< 1.25")
	if obj.Spec.Kubernetes.AllowPrivilegedContainers == nil && k8sLess125 && !isPSPDisabled(obj) {
		obj.Spec.Kubernetes.AllowPrivilegedContainers = pointer.Bool(true)
	}
	if obj.Spec.Kubernetes.KubeAPIServer == nil {
		obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{}
	}
	if obj.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication == nil {
		obj.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = pointer.Bool(false)
	}
	if obj.Spec.Kubernetes.KubeAPIServer.Requests == nil {
		obj.Spec.Kubernetes.KubeAPIServer.Requests = &KubeAPIServerRequests{}
	}
	if obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxNonMutatingInflight == nil {
		obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxNonMutatingInflight = pointer.Int32(400)
	}
	if obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxMutatingInflight == nil {
		obj.Spec.Kubernetes.KubeAPIServer.Requests.MaxMutatingInflight = pointer.Int32(200)
	}
	if obj.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication == nil {
		obj.Spec.Kubernetes.KubeAPIServer.EnableAnonymousAuthentication = pointer.Bool(false)
	}
	if obj.Spec.Kubernetes.KubeAPIServer.EventTTL == nil {
		obj.Spec.Kubernetes.KubeAPIServer.EventTTL = &metav1.Duration{Duration: time.Hour}
	}
	if obj.Spec.Kubernetes.KubeAPIServer.Logging == nil {
		obj.Spec.Kubernetes.KubeAPIServer.Logging = &KubeAPIServerLogging{}
	}
	if obj.Spec.Kubernetes.KubeAPIServer.Logging.Verbosity == nil {
		obj.Spec.Kubernetes.KubeAPIServer.Logging.Verbosity = pointer.Int32(2)
	}
	if obj.Spec.Kubernetes.KubeAPIServer.DefaultNotReadyTolerationSeconds == nil {
		obj.Spec.Kubernetes.KubeAPIServer.DefaultNotReadyTolerationSeconds = pointer.Int64(300)
	}
	if obj.Spec.Kubernetes.KubeAPIServer.DefaultUnreachableTolerationSeconds == nil {
		obj.Spec.Kubernetes.KubeAPIServer.DefaultUnreachableTolerationSeconds = pointer.Int64(300)
	}

	if obj.Spec.Kubernetes.KubeControllerManager == nil {
		obj.Spec.Kubernetes.KubeControllerManager = &KubeControllerManagerConfig{}
	}
	if obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize == nil {
		obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = calculateDefaultNodeCIDRMaskSize(&obj.Spec)
	}
	if obj.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod == nil {
		obj.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod = &metav1.Duration{Duration: 2 * time.Minute}
	}

	if obj.Spec.Kubernetes.KubeScheduler == nil {
		obj.Spec.Kubernetes.KubeScheduler = &KubeSchedulerConfig{}
	}
	if obj.Spec.Kubernetes.KubeScheduler.Profile == nil {
		defaultProfile := SchedulingProfileBalanced
		obj.Spec.Kubernetes.KubeScheduler.Profile = &defaultProfile
	}

	if obj.Spec.Kubernetes.KubeProxy == nil {
		obj.Spec.Kubernetes.KubeProxy = &KubeProxyConfig{}
	}
	if obj.Spec.Kubernetes.KubeProxy.Mode == nil {
		defaultProxyMode := ProxyModeIPTables
		obj.Spec.Kubernetes.KubeProxy.Mode = &defaultProxyMode
	}
	if obj.Spec.Kubernetes.KubeProxy.Enabled == nil {
		obj.Spec.Kubernetes.KubeProxy.Enabled = pointer.Bool(true)
	}

	if obj.Spec.Kubernetes.EnableStaticTokenKubeconfig == nil {
		// Error is ignored here because we cannot do anything meaningful with it - variable will default to "false".
		if k8sLessThan126, _ := versionutils.CheckVersionMeetsConstraint(obj.Spec.Kubernetes.Version, "< 1.26"); k8sLessThan126 {
			obj.Spec.Kubernetes.EnableStaticTokenKubeconfig = pointer.Bool(true)
		} else {
			obj.Spec.Kubernetes.EnableStaticTokenKubeconfig = pointer.Bool(false)
		}
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

	// In previous Gardener versions that weren't supporting tolerations, it was hard-coded to (only) allow shoots in the
	// `garden` namespace to use seeds that had the 'protected' taint. In order to be backwards compatible, now with the
	// introduction of tolerations, we add the 'protected' toleration to the garden namespace by default.
	if obj.Namespace == v1beta1constants.GardenNamespace {
		addTolerations(&obj.Spec.Tolerations, Toleration{Key: SeedTaintProtected})
	}

	if obj.Spec.Kubernetes.Kubelet == nil {
		obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
	}
	if obj.Spec.Kubernetes.Kubelet.FailSwapOn == nil {
		obj.Spec.Kubernetes.Kubelet.FailSwapOn = pointer.Bool(true)
	}
	if obj.Spec.Kubernetes.Kubelet.ImageGCHighThresholdPercent == nil {
		obj.Spec.Kubernetes.Kubelet.ImageGCHighThresholdPercent = pointer.Int32(50)
	}
	if obj.Spec.Kubernetes.Kubelet.ImageGCLowThresholdPercent == nil {
		obj.Spec.Kubernetes.Kubelet.ImageGCLowThresholdPercent = pointer.Int32(40)
	}
	if obj.Spec.Kubernetes.Kubelet.SerializeImagePulls == nil {
		obj.Spec.Kubernetes.Kubelet.SerializeImagePulls = pointer.Bool(true)
	}

	var (
		kubeReservedMemory = resource.MustParse("1Gi")
		kubeReservedCPU    = resource.MustParse("80m")
		kubeReservedPID    = resource.MustParse("20k")
	)

	if obj.Spec.Kubernetes.Kubelet.KubeReserved == nil {
		obj.Spec.Kubernetes.Kubelet.KubeReserved = &KubeletConfigReserved{Memory: &kubeReservedMemory, CPU: &kubeReservedCPU}
		obj.Spec.Kubernetes.Kubelet.KubeReserved.PID = &kubeReservedPID
	} else {
		if obj.Spec.Kubernetes.Kubelet.KubeReserved.Memory == nil {
			obj.Spec.Kubernetes.Kubelet.KubeReserved.Memory = &kubeReservedMemory
		}
		if obj.Spec.Kubernetes.Kubelet.KubeReserved.CPU == nil {
			obj.Spec.Kubernetes.Kubelet.KubeReserved.CPU = &kubeReservedCPU
		}
		if obj.Spec.Kubernetes.Kubelet.KubeReserved.PID == nil {
			obj.Spec.Kubernetes.Kubelet.KubeReserved.PID = &kubeReservedPID
		}
	}

	if obj.Spec.Maintenance == nil {
		obj.Spec.Maintenance = &Maintenance{}
	}

	for i, worker := range obj.Spec.Provider.Workers {
		kubernetesVersion := obj.Spec.Kubernetes.Version
		if worker.Kubernetes != nil && worker.Kubernetes.Version != nil {
			kubernetesVersion = *worker.Kubernetes.Version
		}

		if worker.Machine.Architecture == nil {
			obj.Spec.Provider.Workers[i].Machine.Architecture = pointer.String(v1beta1constants.ArchitectureAMD64)
		}

		if k8sVersionGreaterOrEqualThan122, _ := versionutils.CompareVersions(kubernetesVersion, ">=", "1.22"); !k8sVersionGreaterOrEqualThan122 {
			// Error is ignored here because we cannot do anything meaningful with it.
			// k8sVersionGreaterOrEqualThan122 will default to `false`.
			continue
		}

		if worker.CRI == nil {
			obj.Spec.Provider.Workers[i].CRI = &CRI{Name: CRINameContainerD}
		}
	}

	if obj.Spec.Provider.WorkersSettings == nil {
		obj.Spec.Provider.WorkersSettings = &WorkersSettings{}
	}
	if obj.Spec.Provider.WorkersSettings.SSHAccess == nil {
		obj.Spec.Provider.WorkersSettings.SSHAccess = &SSHAccess{Enabled: true}
	}

	if obj.Spec.SystemComponents == nil {
		obj.Spec.SystemComponents = &SystemComponents{}
	}
	if obj.Spec.SystemComponents.CoreDNS == nil {
		obj.Spec.SystemComponents.CoreDNS = &CoreDNS{}
	}
	if obj.Spec.SystemComponents.CoreDNS.Autoscaling == nil {
		obj.Spec.SystemComponents.CoreDNS.Autoscaling = &CoreDNSAutoscaling{}
	}
	if obj.Spec.SystemComponents.CoreDNS.Autoscaling.Mode != CoreDNSAutoscalingModeHorizontal && obj.Spec.SystemComponents.CoreDNS.Autoscaling.Mode != CoreDNSAutoscalingModeClusterProportional {
		obj.Spec.SystemComponents.CoreDNS.Autoscaling.Mode = CoreDNSAutoscalingModeHorizontal
	}
}

// SetDefaults_Networking sets default values for Networking objects.
func SetDefaults_Networking(obj *Networking) {
	if len(obj.IPFamilies) == 0 {
		obj.IPFamilies = []IPFamily{IPFamilyIPv4}
	}
}

// SetDefaults_Maintenance sets default values for Maintenance objects.
func SetDefaults_Maintenance(obj *Maintenance) {
	if obj.AutoUpdate == nil {
		obj.AutoUpdate = &MaintenanceAutoUpdate{
			KubernetesVersion:   true,
			MachineImageVersion: true,
		}
	}

	if obj.TimeWindow == nil {
		mt := timewindow.RandomMaintenanceTimeWindow()
		obj.TimeWindow = &MaintenanceTimeWindow{
			Begin: mt.Begin().Formatted(),
			End:   mt.End().Formatted(),
		}
	}
}

// SetDefaults_VerticalPodAutoscaler sets default values for VerticalPodAutoscaler objects.
func SetDefaults_VerticalPodAutoscaler(obj *VerticalPodAutoscaler) {
	if obj.EvictAfterOOMThreshold == nil {
		v := DefaultEvictAfterOOMThreshold
		obj.EvictAfterOOMThreshold = &v
	}
	if obj.EvictionRateBurst == nil {
		v := DefaultEvictionRateBurst
		obj.EvictionRateBurst = &v
	}
	if obj.EvictionRateLimit == nil {
		v := DefaultEvictionRateLimit
		obj.EvictionRateLimit = &v
	}
	if obj.EvictionTolerance == nil {
		v := DefaultEvictionTolerance
		obj.EvictionTolerance = &v
	}
	if obj.RecommendationMarginFraction == nil {
		v := DefaultRecommendationMarginFraction
		obj.RecommendationMarginFraction = &v
	}
	if obj.UpdaterInterval == nil {
		v := DefaultUpdaterInterval
		obj.UpdaterInterval = &v
	}
	if obj.RecommenderInterval == nil {
		v := DefaultRecommenderInterval
		obj.RecommenderInterval = &v
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
	if obj.SystemComponents == nil {
		obj.SystemComponents = &WorkerSystemComponents{
			Allow: DefaultWorkerSystemComponentsAllow,
		}
	}
}

// SetDefaults_ClusterAutoscaler sets default values for ClusterAutoscaler object.
func SetDefaults_ClusterAutoscaler(obj *ClusterAutoscaler) {
	if obj.ScaleDownDelayAfterAdd == nil {
		obj.ScaleDownDelayAfterAdd = &metav1.Duration{Duration: 1 * time.Hour}
	}
	if obj.ScaleDownDelayAfterDelete == nil {
		obj.ScaleDownDelayAfterDelete = &metav1.Duration{Duration: 0}
	}
	if obj.ScaleDownDelayAfterFailure == nil {
		obj.ScaleDownDelayAfterFailure = &metav1.Duration{Duration: 3 * time.Minute}
	}
	if obj.ScaleDownUnneededTime == nil {
		obj.ScaleDownUnneededTime = &metav1.Duration{Duration: 30 * time.Minute}
	}
	if obj.ScaleDownUtilizationThreshold == nil {
		obj.ScaleDownUtilizationThreshold = pointer.Float64(0.5)
	}
	if obj.ScanInterval == nil {
		obj.ScanInterval = &metav1.Duration{Duration: 10 * time.Second}
	}
	if obj.Expander == nil {
		leastWaste := ClusterAutoscalerExpanderLeastWaste
		obj.Expander = &leastWaste
	}
	if obj.MaxNodeProvisionTime == nil {
		obj.MaxNodeProvisionTime = &metav1.Duration{Duration: 20 * time.Minute}
	}
	if obj.MaxGracefulTerminationSeconds == nil {
		obj.MaxGracefulTerminationSeconds = pointer.Int32(600)
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

func calculateDefaultNodeCIDRMaskSize(shoot *ShootSpec) *int32 {
	if IsIPv6SingleStack(shoot.Networking.IPFamilies) {
		// If shoot is using IPv6 single-stack, don't be stingy and allocate larger pod CIDRs per node.
		// We don't calculate a nodeCIDRMaskSize matching the maxPods settings in this case, and simply apply
		// kube-controller-manager's default value for the --node-cidr-mask-size flag.
		return pointer.Int32(64)
	}

	var maxPods int32 = 110 // default maxPods setting on kubelet

	if shoot != nil && shoot.Kubernetes.Kubelet != nil && shoot.Kubernetes.Kubelet.MaxPods != nil {
		maxPods = *shoot.Kubernetes.Kubelet.MaxPods
	}

	for _, worker := range shoot.Provider.Workers {
		if worker.Kubernetes != nil && worker.Kubernetes.Kubelet != nil && worker.Kubernetes.Kubelet.MaxPods != nil && *worker.Kubernetes.Kubelet.MaxPods > maxPods {
			maxPods = *worker.Kubernetes.Kubelet.MaxPods
		}
	}

	// by having approximately twice as many available IP addresses as possible Pods, Kubernetes is able to mitigate IP address reuse as Pods are added to and removed from a node.
	nodeCidrRange := int32(32 - int(math.Ceil(math.Log2(float64(maxPods*2)))))
	return &nodeCidrRange
}

func addTolerations(tolerations *[]Toleration, additionalTolerations ...Toleration) {
	existingTolerations := map[Toleration]struct{}{}
	for _, toleration := range *tolerations {
		existingTolerations[toleration] = struct{}{}
	}

	for _, toleration := range additionalTolerations {
		if _, ok := existingTolerations[Toleration{Key: toleration.Key}]; ok {
			continue
		}
		if _, ok := existingTolerations[toleration]; ok {
			continue
		}
		*tolerations = append(*tolerations, toleration)
	}
}

func isPSPDisabled(shoot *Shoot) bool {
	if shoot.Spec.Kubernetes.KubeAPIServer != nil {
		for _, plugin := range shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins {
			if plugin.Name == "PodSecurityPolicy" && pointer.BoolDeref(plugin.Disabled, false) {
				return true
			}
		}
	}
	return false
}
