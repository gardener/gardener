// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package kubelet

import (
	"time"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/utils/version"
)

// Config returns a kubelet config based on the provided parameters and for the provided Kubernetes version.
func Config(kubernetesVersion *semver.Version, clusterDNSAddress, clusterDomain string, params components.ConfigurableKubeletConfigParameters) *kubeletconfigv1beta1.KubeletConfiguration {
	setConfigDefaults(&params, kubernetesVersion)

	config := &kubeletconfigv1beta1.KubeletConfiguration{
		Authentication: kubeletconfigv1beta1.KubeletAuthentication{
			Anonymous: kubeletconfigv1beta1.KubeletAnonymousAuthentication{
				Enabled: pointer.Bool(false),
			},
			X509: kubeletconfigv1beta1.KubeletX509Authentication{
				ClientCAFile: PathKubeletCACert,
			},
			Webhook: kubeletconfigv1beta1.KubeletWebhookAuthentication{
				Enabled:  pointer.Bool(true),
				CacheTTL: metav1.Duration{Duration: 2 * time.Minute},
			},
		},
		Authorization: kubeletconfigv1beta1.KubeletAuthorization{
			Mode: kubeletconfigv1beta1.KubeletAuthorizationModeWebhook,
			Webhook: kubeletconfigv1beta1.KubeletWebhookAuthorization{
				CacheAuthorizedTTL:   metav1.Duration{Duration: 5 * time.Minute},
				CacheUnauthorizedTTL: metav1.Duration{Duration: 30 * time.Second},
			},
		},
		CgroupDriver:                     getCgroupDriver(params),
		CgroupRoot:                       "/",
		CgroupsPerQOS:                    pointer.Bool(true),
		ClusterDNS:                       []string{clusterDNSAddress},
		ClusterDomain:                    clusterDomain,
		ContainerLogMaxSize:              *params.ContainerLogMaxSize,
		ContainerLogMaxFiles:             params.ContainerLogMaxFiles,
		CPUCFSQuota:                      params.CpuCFSQuota,
		CPUManagerPolicy:                 *params.CpuManagerPolicy,
		CPUManagerReconcilePeriod:        metav1.Duration{Duration: 10 * time.Second},
		EnableControllerAttachDetach:     pointer.Bool(true),
		EnableDebuggingHandlers:          pointer.Bool(true),
		EnableServer:                     pointer.Bool(true),
		EnforceNodeAllocatable:           []string{"pods"},
		EventBurst:                       50,
		EventRecordQPS:                   pointer.Int32(50),
		EvictionHard:                     params.EvictionHard,
		EvictionMinimumReclaim:           params.EvictionMinimumReclaim,
		EvictionSoft:                     params.EvictionSoft,
		EvictionSoftGracePeriod:          params.EvictionSoftGracePeriod,
		EvictionPressureTransitionPeriod: *params.EvictionPressureTransitionPeriod,
		EvictionMaxPodGracePeriod:        *params.EvictionMaxPodGracePeriod,
		FailSwapOn:                       params.FailSwapOn,
		FeatureGates:                     params.FeatureGates,
		FileCheckFrequency:               metav1.Duration{Duration: 20 * time.Second},
		HairpinMode:                      kubeletconfigv1beta1.PromiscuousBridge,
		HTTPCheckFrequency:               metav1.Duration{Duration: 20 * time.Second},
		ImageGCHighThresholdPercent:      params.ImageGCHighThresholdPercent,
		ImageGCLowThresholdPercent:       params.ImageGCLowThresholdPercent,
		ImageMinimumGCAge:                metav1.Duration{Duration: 2 * time.Minute},
		KubeAPIBurst:                     50,
		KubeAPIQPS:                       pointer.Int32(50),
		KubeReserved:                     params.KubeReserved,
		MaxOpenFiles:                     1000000,
		MaxPods:                          *params.MaxPods,
		PodsPerCore:                      0,
		PodPidsLimit:                     params.PodPidsLimit,
		ProtectKernelDefaults:            *params.ProtectKernelDefaults,
		ReadOnlyPort:                     0,
		ResolverConfig:                   pointer.String("/etc/resolv.conf"),
		RotateCertificates:               true,
		RuntimeRequestTimeout:            metav1.Duration{Duration: 2 * time.Minute},
		SeccompDefault:                   params.SeccompDefault,
		SerializeImagePulls:              params.SerializeImagePulls,
		ServerTLSBootstrap:               true,
		StreamingConnectionIdleTimeout:   *params.StreamingConnectionIdleTimeout,
		RegisterWithTaints: []corev1.Taint{{
			Key:    v1beta1constants.TaintNodeCriticalComponentsNotReady,
			Effect: corev1.TaintEffectNoSchedule,
		}},
		RegistryPullQPS:      params.RegistryPullQPS,
		RegistryBurst:        pointer.Int32Deref(params.RegistryBurst, 0),
		SyncFrequency:        metav1.Duration{Duration: time.Minute},
		SystemReserved:       params.SystemReserved,
		VolumeStatsAggPeriod: metav1.Duration{Duration: time.Minute},
		VolumePluginDir:      pathVolumePluginDirectory,
	}

	if params.MemorySwap != nil {
		config.MemorySwap = *params.MemorySwap
	}

	return config
}

var (
	evictionHardDefaults = map[string]string{
		components.MemoryAvailable:   "100Mi",
		components.ImageFSAvailable:  "5%",
		components.ImageFSInodesFree: "5%",
		components.NodeFSAvailable:   "5%",
		components.NodeFSInodesFree:  "5%",
	}
	evictionMinimumReclaimDefaults = map[string]string{
		components.MemoryAvailable:   "0Mi",
		components.ImageFSAvailable:  "0Mi",
		components.ImageFSInodesFree: "0Mi",
		components.NodeFSAvailable:   "0Mi",
		components.NodeFSInodesFree:  "0Mi",
	}
	evictionSoftDefaults = map[string]string{
		components.MemoryAvailable:   "200Mi",
		components.ImageFSAvailable:  "10%",
		components.ImageFSInodesFree: "10%",
		components.NodeFSAvailable:   "10%",
		components.NodeFSInodesFree:  "10%",
	}
	evictionSoftGracePeriodDefaults = map[string]string{
		components.MemoryAvailable:   "1m30s",
		components.ImageFSAvailable:  "1m30s",
		components.ImageFSInodesFree: "1m30s",
		components.NodeFSAvailable:   "1m30s",
		components.NodeFSInodesFree:  "1m30s",
	}
	kubeReservedDefaults = map[string]string{
		string(corev1.ResourceCPU):    "80m",
		string(corev1.ResourceMemory): "1Gi",
	}
)

// ShouldProtectKernelDefaultsBeEnabled returns true if ProtectKernelDefaults is set to true in the kubelet's config parameters or k8s version is >= 1.26.
func ShouldProtectKernelDefaultsBeEnabled(kubeletConfigParameters *components.ConfigurableKubeletConfigParameters, kubernetesVersion *semver.Version) bool {
	if kubeletConfigParameters.ProtectKernelDefaults != nil {
		return *kubeletConfigParameters.ProtectKernelDefaults
	}
	return kubernetesVersion != nil && version.ConstraintK8sGreaterEqual126.Check(kubernetesVersion)
}

func getCgroupDriver(kubeletConfigParameters components.ConfigurableKubeletConfigParameters) string {
	cgroupDriver := "cgroupfs"

	// When KubeletCgroupDriverFromCRI feature gate is active CgroupDriver does not have to be specified: https://github.com/kubernetes/kubernetes/pull/118770
	if kubeletConfigParameters.FeatureGates["KubeletCgroupDriverFromCRI"] {
		cgroupDriver = ""
	}

	return cgroupDriver
}

func setConfigDefaults(c *components.ConfigurableKubeletConfigParameters, kubernetesVersion *semver.Version) {
	if c.CpuCFSQuota == nil {
		c.CpuCFSQuota = pointer.Bool(true)
	}

	if c.CpuManagerPolicy == nil {
		c.CpuManagerPolicy = pointer.String(kubeletconfigv1beta1.NoneTopologyManagerPolicy)
	}

	if c.EvictionHard == nil {
		c.EvictionHard = make(map[string]string, 5)
	}
	for k, v := range evictionHardDefaults {
		if c.EvictionHard[k] == "" {
			c.EvictionHard[k] = v
		}
	}

	if c.EvictionSoft == nil {
		c.EvictionSoft = make(map[string]string, 5)
	}
	for k, v := range evictionSoftDefaults {
		if c.EvictionSoft[k] == "" {
			c.EvictionSoft[k] = v
		}
	}

	if c.EvictionSoftGracePeriod == nil {
		c.EvictionSoftGracePeriod = make(map[string]string, 5)
	}
	for k, v := range evictionSoftGracePeriodDefaults {
		if c.EvictionSoftGracePeriod[k] == "" {
			c.EvictionSoftGracePeriod[k] = v
		}
	}

	if c.EvictionMinimumReclaim == nil {
		c.EvictionMinimumReclaim = make(map[string]string, 5)
	}
	for k, v := range evictionMinimumReclaimDefaults {
		if c.EvictionMinimumReclaim[k] == "" {
			c.EvictionMinimumReclaim[k] = v
		}
	}

	if c.EvictionPressureTransitionPeriod == nil {
		c.EvictionPressureTransitionPeriod = &metav1.Duration{Duration: 4 * time.Minute}
	}

	if c.EvictionMaxPodGracePeriod == nil {
		c.EvictionMaxPodGracePeriod = pointer.Int32(90)
	}

	if c.FailSwapOn == nil {
		c.FailSwapOn = pointer.Bool(true)
	}

	if c.ImageGCHighThresholdPercent == nil {
		c.ImageGCHighThresholdPercent = pointer.Int32(50)
	}

	if c.ImageGCLowThresholdPercent == nil {
		c.ImageGCLowThresholdPercent = pointer.Int32(40)
	}

	if c.SerializeImagePulls == nil {
		c.SerializeImagePulls = pointer.Bool(true)
	}

	if c.KubeReserved == nil {
		c.KubeReserved = make(map[string]string, 2)
	}
	for k, v := range kubeReservedDefaults {
		if c.KubeReserved[k] == "" {
			c.KubeReserved[k] = v
		}
	}

	if c.MaxPods == nil {
		c.MaxPods = pointer.Int32(110)
	}

	if c.ContainerLogMaxSize == nil {
		c.ContainerLogMaxSize = pointer.String("100Mi")
	}

	c.ProtectKernelDefaults = pointer.Bool(ShouldProtectKernelDefaultsBeEnabled(c, kubernetesVersion))

	if c.StreamingConnectionIdleTimeout == nil {
		if version.ConstraintK8sGreaterEqual126.Check(kubernetesVersion) {
			c.StreamingConnectionIdleTimeout = &metav1.Duration{Duration: time.Minute * 5}
		} else {
			// this is also the kubernetes default
			c.StreamingConnectionIdleTimeout = &metav1.Duration{Duration: time.Hour * 4}
		}
	}
}
