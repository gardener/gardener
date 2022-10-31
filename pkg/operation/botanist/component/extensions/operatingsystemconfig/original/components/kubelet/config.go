// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubelet

import (
	"time"

	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"
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
		CgroupDriver:                     "cgroupfs",
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
		NodeStatusUpdateFrequency:        metav1.Duration{Duration: 10 * time.Second},
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
		RegistryPullQPS:                  params.RegistryPullQPS,
		RegistryBurst:                    pointer.Int32Deref(params.RegistryBurst, 0),
		SyncFrequency:                    metav1.Duration{Duration: time.Minute},
		SystemReserved:                   params.SystemReserved,
		VolumeStatsAggPeriod:             metav1.Duration{Duration: time.Minute},
	}

	if !version.ConstraintK8sLess119.Check(kubernetesVersion) {
		config.VolumePluginDir = pathVolumePluginDirectory
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

	if c.ProtectKernelDefaults == nil {
		c.ProtectKernelDefaults = pointer.Bool(version.ConstraintK8sGreaterEqual126.Check(kubernetesVersion))
	}

	if c.StreamingConnectionIdleTimeout == nil {
		if version.ConstraintK8sGreaterEqual126.Check(kubernetesVersion) {
			c.StreamingConnectionIdleTimeout = &metav1.Duration{Duration: time.Minute * 5}
		} else {
			// this is also the kubernetes default
			c.StreamingConnectionIdleTimeout = &metav1.Duration{Duration: time.Hour * 4}
		}
	}
}
