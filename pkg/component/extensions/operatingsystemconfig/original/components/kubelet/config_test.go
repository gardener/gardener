// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubelet_test

import (
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Config", func() {
	var (
		clusterDNSAddresses = []string{"foo", "bar"}
		clusterDomain       = "bar"
		params              = components.ConfigurableKubeletConfigParameters{
			ContainerLogMaxSize:              ptr.To("123Mi"),
			CpuCFSQuota:                      ptr.To(false),
			CpuManagerPolicy:                 ptr.To("policy"),
			EvictionHard:                     map[string]string{"memory.available": "123"},
			EvictionMinimumReclaim:           map[string]string{"imagefs.available": "123"},
			EvictionSoft:                     map[string]string{"imagefs.inodesFree": "123"},
			EvictionSoftGracePeriod:          map[string]string{"nodefs.available": "123"},
			EvictionPressureTransitionPeriod: &metav1.Duration{Duration: 42 * time.Minute},
			EvictionMaxPodGracePeriod:        ptr.To[int32](120),
			FailSwapOn:                       ptr.To(false),
			FeatureGates:                     map[string]bool{"Foo": false},
			ImageGCHighThresholdPercent:      ptr.To[int32](34),
			ImageGCLowThresholdPercent:       ptr.To[int32](12),
			ProtectKernelDefaults:            ptr.To(true),
			SeccompDefault:                   ptr.To(true),
			SerializeImagePulls:              ptr.To(true),
			RegistryPullQPS:                  ptr.To[int32](10),
			RegistryBurst:                    ptr.To[int32](20),
			KubeReserved:                     map[string]string{"cpu": "123"},
			MaxPods:                          ptr.To[int32](24),
			MemorySwap:                       &kubeletconfigv1beta1.MemorySwapConfiguration{SwapBehavior: "UnlimitedSwap"},
			PodPidsLimit:                     ptr.To[int64](101),
			SystemReserved:                   map[string]string{"memory": "321"},
			StreamingConnectionIdleTimeout:   &metav1.Duration{Duration: time.Minute * 12},
			WithStaticPodPath:                true,
		}

		taints = []corev1.Taint{{
			Key:    "foo",
			Value:  "bar",
			Effect: corev1.TaintEffectNoSchedule,
		}}

		kubeletConfigWithDefaults = &kubeletconfigv1beta1.KubeletConfiguration{
			Authentication: kubeletconfigv1beta1.KubeletAuthentication{
				Anonymous: kubeletconfigv1beta1.KubeletAnonymousAuthentication{
					Enabled: ptr.To(false),
				},
				X509: kubeletconfigv1beta1.KubeletX509Authentication{
					ClientCAFile: "/var/lib/kubelet/ca.crt",
				},
				Webhook: kubeletconfigv1beta1.KubeletWebhookAuthentication{
					Enabled:  ptr.To(true),
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
			CgroupDriver:                 "cgroupfs",
			CgroupRoot:                   "/",
			CgroupsPerQOS:                ptr.To(true),
			ClusterDNS:                   clusterDNSAddresses,
			ClusterDomain:                clusterDomain,
			ContainerLogMaxSize:          "100Mi",
			CPUCFSQuota:                  ptr.To(true),
			CPUManagerPolicy:             "none",
			CPUManagerReconcilePeriod:    metav1.Duration{Duration: 10 * time.Second},
			EnableControllerAttachDetach: ptr.To(true),
			EnableDebuggingHandlers:      ptr.To(true),
			EnableServer:                 ptr.To(true),
			EnforceNodeAllocatable:       []string{"pods"},
			EventBurst:                   50,
			EventRecordQPS:               ptr.To[int32](50),
			EvictionHard: map[string]string{
				"memory.available":   "100Mi",
				"imagefs.available":  "5%",
				"imagefs.inodesFree": "5%",
				"nodefs.available":   "5%",
				"nodefs.inodesFree":  "5%",
			},
			EvictionMinimumReclaim: map[string]string{
				"memory.available":   "0Mi",
				"imagefs.available":  "0Mi",
				"imagefs.inodesFree": "0Mi",
				"nodefs.available":   "0Mi",
				"nodefs.inodesFree":  "0Mi",
			},
			EvictionSoft: map[string]string{
				"memory.available":   "200Mi",
				"imagefs.available":  "10%",
				"imagefs.inodesFree": "10%",
				"nodefs.available":   "10%",
				"nodefs.inodesFree":  "10%",
			},
			EvictionSoftGracePeriod: map[string]string{
				"memory.available":   "1m30s",
				"imagefs.available":  "1m30s",
				"imagefs.inodesFree": "1m30s",
				"nodefs.available":   "1m30s",
				"nodefs.inodesFree":  "1m30s",
			},
			EvictionPressureTransitionPeriod: metav1.Duration{Duration: 4 * time.Minute},
			EvictionMaxPodGracePeriod:        90,
			FailSwapOn:                       ptr.To(true),
			FileCheckFrequency:               metav1.Duration{Duration: 20 * time.Second},
			HairpinMode:                      kubeletconfigv1beta1.PromiscuousBridge,
			HTTPCheckFrequency:               metav1.Duration{Duration: 20 * time.Second},
			ImageGCHighThresholdPercent:      ptr.To[int32](50),
			ImageGCLowThresholdPercent:       ptr.To[int32](40),
			ImageMinimumGCAge:                metav1.Duration{Duration: 2 * time.Minute},
			KubeAPIBurst:                     50,
			KubeAPIQPS:                       ptr.To[int32](50),
			KubeReserved: map[string]string{
				"cpu":    "80m",
				"memory": "1Gi",
			},
			MaxOpenFiles:   1000000,
			MaxPods:        110,
			PodsPerCore:    0,
			ReadOnlyPort:   0,
			ResolverConfig: ptr.To("/etc/resolv.conf"),
			RegisterWithTaints: append(taints,
				corev1.Taint{
					Key:    "node.gardener.cloud/critical-components-not-ready",
					Effect: corev1.TaintEffectNoSchedule,
				}),
			RuntimeRequestTimeout:          metav1.Duration{Duration: 2 * time.Minute},
			SerializeImagePulls:            ptr.To(true),
			ServerTLSBootstrap:             true,
			StaticPodPath:                  "/etc/kubernetes/manifests",
			StreamingConnectionIdleTimeout: metav1.Duration{Duration: time.Hour * 4},
			SyncFrequency:                  metav1.Duration{Duration: time.Minute},
			VolumeStatsAggPeriod:           metav1.Duration{Duration: time.Minute},
		}

		kubeletConfigWithParams = &kubeletconfigv1beta1.KubeletConfiguration{
			Authentication: kubeletconfigv1beta1.KubeletAuthentication{
				Anonymous: kubeletconfigv1beta1.KubeletAnonymousAuthentication{
					Enabled: ptr.To(false),
				},
				X509: kubeletconfigv1beta1.KubeletX509Authentication{
					ClientCAFile: "/var/lib/kubelet/ca.crt",
				},
				Webhook: kubeletconfigv1beta1.KubeletWebhookAuthentication{
					Enabled:  ptr.To(true),
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
			CgroupDriver:                 "cgroupfs",
			CgroupRoot:                   "/",
			CgroupsPerQOS:                ptr.To(true),
			ClusterDomain:                clusterDomain,
			ClusterDNS:                   clusterDNSAddresses,
			ContainerLogMaxSize:          "123Mi",
			CPUCFSQuota:                  params.CpuCFSQuota,
			CPUManagerPolicy:             *params.CpuManagerPolicy,
			CPUManagerReconcilePeriod:    metav1.Duration{Duration: 10 * time.Second},
			EnableControllerAttachDetach: ptr.To(true),
			EnableDebuggingHandlers:      ptr.To(true),
			EnableServer:                 ptr.To(true),
			EnforceNodeAllocatable:       []string{"pods"},
			EventBurst:                   50,
			EventRecordQPS:               ptr.To[int32](50),
			EvictionHard: utils.MergeStringMaps(params.EvictionHard, map[string]string{
				"imagefs.available":  "5%",
				"imagefs.inodesFree": "5%",
				"nodefs.available":   "5%",
				"nodefs.inodesFree":  "5%",
			}),
			EvictionMinimumReclaim: utils.MergeStringMaps(params.EvictionMinimumReclaim, map[string]string{
				"memory.available":   "0Mi",
				"imagefs.inodesFree": "0Mi",
				"nodefs.available":   "0Mi",
				"nodefs.inodesFree":  "0Mi",
			}),
			EvictionSoft: utils.MergeStringMaps(params.EvictionSoft, map[string]string{
				"memory.available":  "200Mi",
				"imagefs.available": "10%",
				"nodefs.available":  "10%",
				"nodefs.inodesFree": "10%",
			}),
			EvictionSoftGracePeriod: utils.MergeStringMaps(params.EvictionSoftGracePeriod, map[string]string{
				"memory.available":   "1m30s",
				"imagefs.available":  "1m30s",
				"imagefs.inodesFree": "1m30s",
				"nodefs.inodesFree":  "1m30s",
			}),
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
			KubeAPIQPS:                       ptr.To[int32](50),
			KubeReserved:                     utils.MergeStringMaps(params.KubeReserved, map[string]string{"memory": "1Gi"}),
			MaxOpenFiles:                     1000000,
			MaxPods:                          *params.MaxPods,
			MemorySwap:                       *params.MemorySwap,
			PodsPerCore:                      0,
			PodPidsLimit:                     params.PodPidsLimit,
			ProtectKernelDefaults:            true,
			ReadOnlyPort:                     0,
			RegisterWithTaints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "node.gardener.cloud/critical-components-not-ready",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			RegistryBurst:                  20,
			RegistryPullQPS:                ptr.To[int32](10),
			ResolverConfig:                 ptr.To("/etc/resolv.conf"),
			RuntimeRequestTimeout:          metav1.Duration{Duration: 2 * time.Minute},
			SerializeImagePulls:            params.SerializeImagePulls,
			SeccompDefault:                 params.SeccompDefault,
			ServerTLSBootstrap:             true,
			StaticPodPath:                  "/etc/kubernetes/manifests",
			SyncFrequency:                  metav1.Duration{Duration: time.Minute},
			SystemReserved:                 params.SystemReserved,
			StreamingConnectionIdleTimeout: metav1.Duration{Duration: time.Minute * 12},
			VolumeStatsAggPeriod:           metav1.Duration{Duration: time.Minute},
		}
	)

	DescribeTable("#Config",
		func(kubernetesVersion string, clusterDNSAddresses []string, clusterDomain string, params components.ConfigurableKubeletConfigParameters, expectedConfig *kubeletconfigv1beta1.KubeletConfiguration, mutateExpectConfigFn func(*kubeletconfigv1beta1.KubeletConfiguration)) {
			expectation := expectedConfig.DeepCopy()
			if mutateExpectConfigFn != nil {
				mutateExpectConfigFn(expectation)
			}

			Expect(kubelet.Config(semver.MustParse(kubernetesVersion), clusterDNSAddresses, clusterDomain, taints, params)).To(DeepEqual(expectation))
		},

		Entry(
			"kubernetes 1.27 w/o defaults",
			"1.27.1",
			clusterDNSAddresses,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
				cfg.ProtectKernelDefaults = true
				cfg.StreamingConnectionIdleTimeout = metav1.Duration{Duration: time.Minute * 5}
				cfg.StaticPodPath = ""
			},
		),
		Entry(
			"kubernetes 1.27 w/ defaults",
			"1.27.1",
			clusterDNSAddresses,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),

		Entry(
			"kubernetes 1.28 w/o defaults",
			"1.28.1",
			clusterDNSAddresses,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
				cfg.ProtectKernelDefaults = true
				cfg.StreamingConnectionIdleTimeout = metav1.Duration{Duration: time.Minute * 5}
				cfg.StaticPodPath = ""
			},
		),
		Entry(
			"kubernetes 1.28 w/ defaults",
			"1.28.1",
			clusterDNSAddresses,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),

		Entry(
			"kubernetes 1.31 w/o defaults",
			"1.31.1",
			clusterDNSAddresses,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.CgroupDriver = "systemd"
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
				cfg.ProtectKernelDefaults = true
				cfg.StreamingConnectionIdleTimeout = metav1.Duration{Duration: time.Minute * 5}
				cfg.StaticPodPath = ""
			},
		),
		Entry(
			"kubernetes 1.31 w/ defaults",
			"1.31.1",
			clusterDNSAddresses,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.CgroupDriver = "systemd"
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),
	)
})
