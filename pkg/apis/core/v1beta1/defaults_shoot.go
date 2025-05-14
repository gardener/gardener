// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"math"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

// SetDefaults_Shoot sets default values for Shoot objects.
func SetDefaults_Shoot(obj *Shoot) {
	if obj.Spec.Kubernetes.KubeAPIServer == nil {
		obj.Spec.Kubernetes.KubeAPIServer = &KubeAPIServerConfig{}
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

	if obj.Spec.Maintenance == nil {
		obj.Spec.Maintenance = &Maintenance{}
	}
	if obj.Spec.Maintenance.AutoUpdate == nil {
		obj.Spec.Maintenance.AutoUpdate = &MaintenanceAutoUpdate{
			KubernetesVersion: true,
		}
	}

	if obj.Spec.Networking == nil {
		obj.Spec.Networking = &Networking{}
	}

	for i, worker := range obj.Spec.Provider.Workers {
		if worker.Machine.Architecture == nil {
			obj.Spec.Provider.Workers[i].Machine.Architecture = ptr.To(v1beta1constants.ArchitectureAMD64)
		}

		if worker.CRI == nil {
			obj.Spec.Provider.Workers[i].CRI = &CRI{Name: CRINameContainerD}
		}

		if worker.Kubernetes != nil && worker.Kubernetes.Kubelet != nil {
			if worker.Kubernetes.Kubelet.FailSwapOn == nil {
				obj.Spec.Provider.Workers[i].Kubernetes.Kubelet.FailSwapOn = ptr.To(true)
			}

			if nodeSwapFeatureGateEnabled, ok := worker.Kubernetes.Kubelet.FeatureGates["NodeSwap"]; ok && nodeSwapFeatureGateEnabled && !*worker.Kubernetes.Kubelet.FailSwapOn {
				if worker.Kubernetes.Kubelet.MemorySwap == nil {
					obj.Spec.Provider.Workers[i].Kubernetes.Kubelet.MemorySwap = &MemorySwapConfiguration{}
				}

				if worker.Kubernetes.Kubelet.MemorySwap.SwapBehavior == nil {
					limitedSwap := LimitedSwap
					obj.Spec.Provider.Workers[i].Kubernetes.Kubelet.MemorySwap.SwapBehavior = &limitedSwap
				}
			}
		}
	}

	// these fields are relevant only for shoot with workers
	if len(obj.Spec.Provider.Workers) > 0 {
		if obj.Spec.Kubernetes.KubeAPIServer.DefaultNotReadyTolerationSeconds == nil {
			obj.Spec.Kubernetes.KubeAPIServer.DefaultNotReadyTolerationSeconds = ptr.To[int64](300)
		}
		if obj.Spec.Kubernetes.KubeAPIServer.DefaultUnreachableTolerationSeconds == nil {
			obj.Spec.Kubernetes.KubeAPIServer.DefaultUnreachableTolerationSeconds = ptr.To[int64](300)
		}

		if obj.Spec.Kubernetes.KubeControllerManager == nil {
			obj.Spec.Kubernetes.KubeControllerManager = &KubeControllerManagerConfig{}
		}

		if obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize == nil {
			obj.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = calculateDefaultNodeCIDRMaskSize(&obj.Spec)
		}

		if obj.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod == nil {
			obj.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod = &metav1.Duration{Duration: 40 * time.Second}
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
			obj.Spec.Kubernetes.KubeProxy.Enabled = ptr.To(true)
		}

		if obj.Spec.Addons == nil {
			obj.Spec.Addons = &Addons{}
		}
		if obj.Spec.Addons.KubernetesDashboard == nil {
			obj.Spec.Addons.KubernetesDashboard = &KubernetesDashboard{}
		}
		if obj.Spec.Addons.KubernetesDashboard.AuthenticationMode == nil {
			defaultAuthMode := KubernetesDashboardAuthModeToken
			obj.Spec.Addons.KubernetesDashboard.AuthenticationMode = &defaultAuthMode
		}

		if obj.Spec.Kubernetes.Kubelet == nil {
			obj.Spec.Kubernetes.Kubelet = &KubeletConfig{}
		}
		if obj.Spec.Kubernetes.Kubelet.FailSwapOn == nil {
			obj.Spec.Kubernetes.Kubelet.FailSwapOn = ptr.To(true)
		}

		if nodeSwapFeatureGateEnabled, ok := obj.Spec.Kubernetes.Kubelet.FeatureGates["NodeSwap"]; ok && nodeSwapFeatureGateEnabled && !*obj.Spec.Kubernetes.Kubelet.FailSwapOn {
			if obj.Spec.Kubernetes.Kubelet.MemorySwap == nil {
				obj.Spec.Kubernetes.Kubelet.MemorySwap = &MemorySwapConfiguration{}
			}
			if obj.Spec.Kubernetes.Kubelet.MemorySwap.SwapBehavior == nil {
				limitedSwap := LimitedSwap
				obj.Spec.Kubernetes.Kubelet.MemorySwap.SwapBehavior = &limitedSwap
			}
		}
		if obj.Spec.Kubernetes.Kubelet.ImageGCHighThresholdPercent == nil {
			obj.Spec.Kubernetes.Kubelet.ImageGCHighThresholdPercent = ptr.To[int32](50)
		}
		if obj.Spec.Kubernetes.Kubelet.ImageGCLowThresholdPercent == nil {
			obj.Spec.Kubernetes.Kubelet.ImageGCLowThresholdPercent = ptr.To[int32](40)
		}
		if obj.Spec.Kubernetes.Kubelet.SerializeImagePulls == nil {
			// SerializeImagePulls defaults to true when MaxParallelImagePulls is not set
			if obj.Spec.Kubernetes.Kubelet.MaxParallelImagePulls == nil || *obj.Spec.Kubernetes.Kubelet.MaxParallelImagePulls < 2 {
				obj.Spec.Kubernetes.Kubelet.SerializeImagePulls = ptr.To(true)
			} else {
				obj.Spec.Kubernetes.Kubelet.SerializeImagePulls = ptr.To(false)
			}
		}

		if obj.Spec.Maintenance.AutoUpdate.MachineImageVersion == nil {
			obj.Spec.Maintenance.AutoUpdate.MachineImageVersion = ptr.To(true)
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

	if obj.Spec.SchedulerName == nil {
		obj.Spec.SchedulerName = ptr.To(v1beta1constants.DefaultSchedulerName)
	}
}

// SetDefaults_KubeAPIServerConfig sets default values for KubeAPIServerConfig objects.
func SetDefaults_KubeAPIServerConfig(obj *KubeAPIServerConfig) {
	if obj.Requests == nil {
		obj.Requests = &APIServerRequests{}
	}
	if obj.Requests.MaxNonMutatingInflight == nil {
		obj.Requests.MaxNonMutatingInflight = ptr.To[int32](400)
	}
	if obj.Requests.MaxMutatingInflight == nil {
		obj.Requests.MaxMutatingInflight = ptr.To[int32](200)
	}
	if obj.EnableAnonymousAuthentication == nil {
		obj.EnableAnonymousAuthentication = ptr.To(false)
	}
	if obj.EventTTL == nil {
		obj.EventTTL = &metav1.Duration{Duration: time.Hour}
	}
	if obj.Logging == nil {
		obj.Logging = &APIServerLogging{}
	}
	if obj.Logging.Verbosity == nil {
		obj.Logging.Verbosity = ptr.To[int32](2)
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
		obj.EvictAfterOOMThreshold = ptr.To(DefaultEvictAfterOOMThreshold)
	}
	if obj.EvictionRateBurst == nil {
		obj.EvictionRateBurst = ptr.To(DefaultEvictionRateBurst)
	}
	if obj.EvictionRateLimit == nil {
		obj.EvictionRateLimit = ptr.To(DefaultEvictionRateLimit)
	}
	if obj.EvictionTolerance == nil {
		obj.EvictionTolerance = ptr.To(DefaultEvictionTolerance)
	}
	if obj.RecommendationMarginFraction == nil {
		obj.RecommendationMarginFraction = ptr.To(DefaultRecommendationMarginFraction)
	}
	if obj.UpdaterInterval == nil {
		obj.UpdaterInterval = ptr.To(DefaultUpdaterInterval)
	}
	if obj.RecommenderInterval == nil {
		obj.RecommenderInterval = ptr.To(DefaultRecommenderInterval)
	}
	if obj.TargetCPUPercentile == nil {
		obj.TargetCPUPercentile = ptr.To(DefaultTargetCPUPercentile)
	}
	if obj.RecommendationLowerBoundCPUPercentile == nil {
		obj.RecommendationLowerBoundCPUPercentile = ptr.To(DefaultRecommendationLowerBoundCPUPercentile)
	}
	if obj.RecommendationUpperBoundCPUPercentile == nil {
		obj.RecommendationUpperBoundCPUPercentile = ptr.To(DefaultRecommendationUpperBoundCPUPercentile)
	}
	if obj.CPUHistogramDecayHalfLife == nil {
		obj.CPUHistogramDecayHalfLife = ptr.To(DefaultCPUHistogramDecayHalfLife)
	}
	if obj.TargetMemoryPercentile == nil {
		obj.TargetMemoryPercentile = ptr.To(DefaultTargetMemoryPercentile)
	}
	if obj.RecommendationLowerBoundMemoryPercentile == nil {
		obj.RecommendationLowerBoundMemoryPercentile = ptr.To(DefaultRecommendationLowerBoundMemoryPercentile)
	}
	if obj.RecommendationUpperBoundMemoryPercentile == nil {
		obj.RecommendationUpperBoundMemoryPercentile = ptr.To(DefaultRecommendationUpperBoundMemoryPercentile)
	}
	if obj.MemoryHistogramDecayHalfLife == nil {
		obj.MemoryHistogramDecayHalfLife = ptr.To(DefaultMemoryHistogramDecayHalfLife)
	}
	if obj.MemoryAggregationInterval == nil {
		obj.MemoryAggregationInterval = ptr.To(DefaultMemoryAggregationInterval)
	}
	if obj.MemoryAggregationIntervalCount == nil {
		obj.MemoryAggregationIntervalCount = ptr.To(DefaultMemoryAggregationIntervalCount)
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
	if obj.UpdateStrategy == nil {
		obj.UpdateStrategy = ptr.To(AutoRollingUpdate)
	}

	if *obj.UpdateStrategy == AutoInPlaceUpdate || *obj.UpdateStrategy == ManualInPlaceUpdate {
		if obj.MachineControllerManagerSettings == nil {
			obj.MachineControllerManagerSettings = &MachineControllerManagerSettings{}
		}
		if obj.MachineControllerManagerSettings.DisableHealthTimeout == nil {
			obj.MachineControllerManagerSettings.DisableHealthTimeout = ptr.To(true)
		}

		// In case of manual in-place update, we set the MaxSurge to 0 and MaxUnavailable to 1.
		if *obj.UpdateStrategy == ManualInPlaceUpdate {
			obj.MaxSurge = ptr.To(intstr.FromInt32(0))
			obj.MaxUnavailable = ptr.To(intstr.FromInt32(1))
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
		obj.ScaleDownUtilizationThreshold = ptr.To(float64(0.5))
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
		obj.MaxGracefulTerminationSeconds = ptr.To[int32](600)
	}
	if obj.IgnoreDaemonsetsUtilization == nil {
		obj.IgnoreDaemonsetsUtilization = ptr.To(false)
	}
	if obj.Verbosity == nil {
		obj.Verbosity = ptr.To[int32](2)
	}
	if obj.NewPodScaleUpDelay == nil {
		obj.NewPodScaleUpDelay = &metav1.Duration{Duration: 0}
	}
	if obj.MaxEmptyBulkDelete == nil {
		obj.MaxEmptyBulkDelete = ptr.To[int32](10)
	}
}

// SetDefaults_NginxIngress sets default values for NginxIngress objects.
func SetDefaults_NginxIngress(obj *NginxIngress) {
	if obj.ExternalTrafficPolicy == nil {
		v := corev1.ServiceExternalTrafficPolicyCluster
		obj.ExternalTrafficPolicy = &v
	}
}

// Helper functions

func calculateDefaultNodeCIDRMaskSize(shoot *ShootSpec) *int32 {
	if IsIPv6SingleStack(shoot.Networking.IPFamilies) {
		// If shoot is using IPv6 single-stack, don't be stingy and allocate larger pod CIDRs per node.
		// We don't calculate a nodeCIDRMaskSize matching the maxPods settings in this case, and simply apply
		// kube-controller-manager's default value for the --node-cidr-mask-size flag.
		return ptr.To[int32](64)
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
	nodeCidrRange := 32 - int32(math.Ceil(math.Log2(float64(maxPods*2))))
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
