// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubelet_test

import (
	"time"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Config", func() {
	var (
		clusterDNSAddress = "foo"
		clusterDomain     = "bar"
		params            = components.ConfigurableKubeletConfigParameters{
			ContainerLogMaxSize:              pointer.String("123Mi"),
			CpuCFSQuota:                      pointer.Bool(false),
			CpuManagerPolicy:                 pointer.String("policy"),
			EvictionHard:                     map[string]string{"memory.available": "123"},
			EvictionMinimumReclaim:           map[string]string{"imagefs.available": "123"},
			EvictionSoft:                     map[string]string{"imagefs.inodesFree": "123"},
			EvictionSoftGracePeriod:          map[string]string{"nodefs.available": "123"},
			EvictionPressureTransitionPeriod: &metav1.Duration{Duration: 42 * time.Minute},
			EvictionMaxPodGracePeriod:        pointer.Int32(120),
			FailSwapOn:                       pointer.Bool(false),
			FeatureGates:                     map[string]bool{"Foo": false},
			ImageGCHighThresholdPercent:      pointer.Int32(34),
			ImageGCLowThresholdPercent:       pointer.Int32(12),
			ProtectKernelDefaults:            pointer.Bool(true),
			SeccompDefault:                   pointer.Bool(true),
			SerializeImagePulls:              pointer.Bool(true),
			RegistryPullQPS:                  pointer.Int32(10),
			RegistryBurst:                    pointer.Int32(20),
			KubeReserved:                     map[string]string{"cpu": "123"},
			MaxPods:                          pointer.Int32(24),
			MemorySwap:                       &kubeletconfigv1beta1.MemorySwapConfiguration{SwapBehavior: "UnlimitedSwap"},
			PodPidsLimit:                     pointer.Int64(101),
			SystemReserved:                   map[string]string{"memory": "321"},
			StreamingConnectionIdleTimeout:   &metav1.Duration{Duration: time.Minute * 12},
		}

		kubeletConfigWithDefaults = &kubeletconfigv1beta1.KubeletConfiguration{
			Authentication: kubeletconfigv1beta1.KubeletAuthentication{
				Anonymous: kubeletconfigv1beta1.KubeletAnonymousAuthentication{
					Enabled: pointer.Bool(false),
				},
				X509: kubeletconfigv1beta1.KubeletX509Authentication{
					ClientCAFile: "/var/lib/kubelet/ca.crt",
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
			CgroupDriver:                 "cgroupfs",
			CgroupRoot:                   "/",
			CgroupsPerQOS:                pointer.Bool(true),
			ClusterDNS:                   []string{clusterDNSAddress},
			ClusterDomain:                clusterDomain,
			ContainerLogMaxSize:          "100Mi",
			CPUCFSQuota:                  pointer.Bool(true),
			CPUManagerPolicy:             "none",
			CPUManagerReconcilePeriod:    metav1.Duration{Duration: 10 * time.Second},
			EnableControllerAttachDetach: pointer.Bool(true),
			EnableDebuggingHandlers:      pointer.Bool(true),
			EnableServer:                 pointer.Bool(true),
			EnforceNodeAllocatable:       []string{"pods"},
			EventBurst:                   50,
			EventRecordQPS:               pointer.Int32(50),
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
			FailSwapOn:                       pointer.Bool(true),
			FileCheckFrequency:               metav1.Duration{Duration: 20 * time.Second},
			HairpinMode:                      kubeletconfigv1beta1.PromiscuousBridge,
			HTTPCheckFrequency:               metav1.Duration{Duration: 20 * time.Second},
			ImageGCHighThresholdPercent:      pointer.Int32(50),
			ImageGCLowThresholdPercent:       pointer.Int32(40),
			ImageMinimumGCAge:                metav1.Duration{Duration: 2 * time.Minute},
			KubeAPIBurst:                     50,
			KubeAPIQPS:                       pointer.Int32(50),
			KubeReserved: map[string]string{
				"cpu":    "80m",
				"memory": "1Gi",
			},
			MaxOpenFiles:   1000000,
			MaxPods:        110,
			PodsPerCore:    0,
			ReadOnlyPort:   0,
			ResolverConfig: pointer.String("/etc/resolv.conf"),
			RegisterWithTaints: []corev1.Taint{{
				Key:    "node.gardener.cloud/critical-components-not-ready",
				Effect: corev1.TaintEffectNoSchedule,
			}},
			RuntimeRequestTimeout:          metav1.Duration{Duration: 2 * time.Minute},
			SerializeImagePulls:            pointer.Bool(true),
			ServerTLSBootstrap:             true,
			StreamingConnectionIdleTimeout: metav1.Duration{Duration: time.Hour * 4},
			SyncFrequency:                  metav1.Duration{Duration: time.Minute},
			VolumeStatsAggPeriod:           metav1.Duration{Duration: time.Minute},
		}

		kubeletConfigWithParams = &kubeletconfigv1beta1.KubeletConfiguration{
			Authentication: kubeletconfigv1beta1.KubeletAuthentication{
				Anonymous: kubeletconfigv1beta1.KubeletAnonymousAuthentication{
					Enabled: pointer.Bool(false),
				},
				X509: kubeletconfigv1beta1.KubeletX509Authentication{
					ClientCAFile: "/var/lib/kubelet/ca.crt",
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
			CgroupDriver:                 "cgroupfs",
			CgroupRoot:                   "/",
			CgroupsPerQOS:                pointer.Bool(true),
			ClusterDomain:                clusterDomain,
			ClusterDNS:                   []string{clusterDNSAddress},
			ContainerLogMaxSize:          "123Mi",
			CPUCFSQuota:                  params.CpuCFSQuota,
			CPUManagerPolicy:             *params.CpuManagerPolicy,
			CPUManagerReconcilePeriod:    metav1.Duration{Duration: 10 * time.Second},
			EnableControllerAttachDetach: pointer.Bool(true),
			EnableDebuggingHandlers:      pointer.Bool(true),
			EnableServer:                 pointer.Bool(true),
			EnforceNodeAllocatable:       []string{"pods"},
			EventBurst:                   50,
			EventRecordQPS:               pointer.Int32(50),
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
			KubeAPIQPS:                       pointer.Int32(50),
			KubeReserved:                     utils.MergeStringMaps(params.KubeReserved, map[string]string{"memory": "1Gi"}),
			MaxOpenFiles:                     1000000,
			MaxPods:                          *params.MaxPods,
			MemorySwap:                       *params.MemorySwap,
			PodsPerCore:                      0,
			PodPidsLimit:                     params.PodPidsLimit,
			ProtectKernelDefaults:            true,
			ReadOnlyPort:                     0,
			RegisterWithTaints: []corev1.Taint{{
				Key:    "node.gardener.cloud/critical-components-not-ready",
				Effect: corev1.TaintEffectNoSchedule,
			}},
			RegistryBurst:                  20,
			RegistryPullQPS:                pointer.Int32(10),
			ResolverConfig:                 pointer.String("/etc/resolv.conf"),
			RuntimeRequestTimeout:          metav1.Duration{Duration: 2 * time.Minute},
			SerializeImagePulls:            params.SerializeImagePulls,
			SeccompDefault:                 params.SeccompDefault,
			ServerTLSBootstrap:             true,
			SyncFrequency:                  metav1.Duration{Duration: time.Minute},
			SystemReserved:                 params.SystemReserved,
			StreamingConnectionIdleTimeout: metav1.Duration{Duration: time.Minute * 12},
			VolumeStatsAggPeriod:           metav1.Duration{Duration: time.Minute},
		}
	)

	DescribeTable("#Config",
		func(kubernetesVersion string, clusterDNSAddress, clusterDomain string, params components.ConfigurableKubeletConfigParameters, expectedConfig *kubeletconfigv1beta1.KubeletConfiguration, mutateExpectConfigFn func(*kubeletconfigv1beta1.KubeletConfiguration)) {
			expectation := expectedConfig.DeepCopy()
			if mutateExpectConfigFn != nil {
				mutateExpectConfigFn(expectation)
			}

			Expect(kubelet.Config(semver.MustParse(kubernetesVersion), clusterDNSAddress, clusterDomain, params)).To(DeepEqual(expectation))
		},

		Entry(
			"kubernetes 1.22 w/o defaults",
			"1.22.1",
			clusterDNSAddress,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),
		Entry(
			"kubernetes 1.22 w/ defaults",
			"1.22.1",
			clusterDNSAddress,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),

		Entry(
			"kubernetes 1.23 w/o defaults",
			"1.23.1",
			clusterDNSAddress,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),
		Entry(
			"kubernetes 1.23 w/ defaults",
			"1.23.1",
			clusterDNSAddress,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),

		Entry(
			"kubernetes 1.24 w/o defaults",
			"1.24.1",
			clusterDNSAddress,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),
		Entry(
			"kubernetes 1.24 w/ defaults",
			"1.24.1",
			clusterDNSAddress,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),

		Entry(
			"kubernetes 1.25 w/o defaults",
			"1.25.1",
			clusterDNSAddress,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),
		Entry(
			"kubernetes 1.25 w/ defaults",
			"1.25.1",
			clusterDNSAddress,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),

		Entry(
			"kubernetes 1.26 w/o defaults",
			"1.26.1",
			clusterDNSAddress,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
				cfg.ProtectKernelDefaults = true
				cfg.StreamingConnectionIdleTimeout = metav1.Duration{Duration: time.Minute * 5}
			},
		),
		Entry(
			"kubernetes 1.26 w/ defaults",
			"1.26.1",
			clusterDNSAddress,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),

		Entry(
			"kubernetes 1.27 w/o defaults",
			"1.27.1",
			clusterDNSAddress,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
				cfg.ProtectKernelDefaults = true
				cfg.StreamingConnectionIdleTimeout = metav1.Duration{Duration: time.Minute * 5}
			},
		),
		Entry(
			"kubernetes 1.27 w/ defaults",
			"1.27.1",
			clusterDNSAddress,
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
			clusterDNSAddress,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
				cfg.ProtectKernelDefaults = true
				cfg.StreamingConnectionIdleTimeout = metav1.Duration{Duration: time.Minute * 5}
			},
		),
		Entry(
			"kubernetes 1.28 w/ defaults",
			"1.28.1",
			clusterDNSAddress,
			clusterDomain,
			params,
			kubeletConfigWithParams,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
			},
		),
		Entry(
			"kubernetes 1.28 w/ KubeletCgroupDriverFromCRI feature gate",
			"1.28.1",
			clusterDNSAddress,
			clusterDomain,
			components.ConfigurableKubeletConfigParameters{FeatureGates: map[string]bool{"KubeletCgroupDriverFromCRI": true}},
			kubeletConfigWithDefaults,
			func(cfg *kubeletconfigv1beta1.KubeletConfiguration) {
				cfg.CgroupDriver = ""
				cfg.FeatureGates = map[string]bool{"KubeletCgroupDriverFromCRI": true}
				cfg.RotateCertificates = true
				cfg.VolumePluginDir = "/var/lib/kubelet/volumeplugins"
				cfg.ProtectKernelDefaults = true
				cfg.StreamingConnectionIdleTimeout = metav1.Duration{Duration: time.Minute * 5}
			},
		),
	)
})
