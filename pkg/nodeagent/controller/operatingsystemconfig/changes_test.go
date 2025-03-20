// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	. "github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
)

var _ = Describe("Changes", func() {
	DescribeTable("#CheckIfMinorVersionUpdate", func(oldVersion, newVersion string, expected bool, errMatcher gomegatypes.GomegaMatcher) {
		isMinorVersionUpdate, err := CheckIfMinorVersionUpdate(oldVersion, newVersion)
		Expect(err).To(errMatcher)
		Expect(isMinorVersionUpdate).To(Equal(expected))
	},
		Entry("invalid version", "a.b.c", "1.0.0", false, MatchError(ContainSubstring("failed to parse"))),
		Entry("1.0.0 to 1.1.0", "1.0.0", "1.1.0", true, BeNil()),
		Entry("1.1.0 to 1.1.2", "1.1.0", "1.1.2", false, BeNil()),
		Entry("v1.1.0 to v1.1.2", "v1.1.0", "v1.1.2", false, BeNil()),
		Entry("v1.2.3 to v1.3.4", "v1.2.3", "v1.3.4", true, BeNil()),
		Entry("v1.3.4 to v1.5.6", "v1.3.4", "v1.5.6", true, BeNil()),
		Entry("1.2.3-foo.12 to 1.2.4-foo.23", "1.2.3-foo.12", "1.2.4-foo.12", false, BeNil()),
		Entry("1.1.1-foo.12 to 1.2.3-foo.23", "1.1.1-foo.12", "1.2.3-foo.23", true, BeNil()),
	)

	DescribeTable("#ComputeKubeletConfigChange", func(oldKubeletConfig, newKubeletConfig *kubeletconfigv1beta1.KubeletConfiguration, expectedCPUManagerPolicyChange, expectedConfigChange bool, errMatcher gomegatypes.GomegaMatcher) {
		configChange, cpuManagerPolicyChange, err := ComputeKubeletConfigChange(oldKubeletConfig, newKubeletConfig)
		Expect(err).To(errMatcher)
		Expect(cpuManagerPolicyChange).To(Equal(expectedCPUManagerPolicyChange))
		Expect(configChange).To(Equal(expectedConfigChange))
	},
		Entry("no change", &kubeletconfigv1beta1.KubeletConfiguration{}, &kubeletconfigv1beta1.KubeletConfiguration{}, false, false, BeNil()),
		Entry("changed cpu manager policy", &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "static"}, &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "none"}, true, false, BeNil()),

		Entry("invalid kubeReserved CPU", &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "static", KubeReserved: map[string]string{"cpu": "aoeu"}, SystemReserved: map[string]string{"cpu": "100m"}}, &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "none", KubeReserved: map[string]string{"cpu": "100m"}}, true, false, MatchError(ContainSubstring("failed to parse"))),
		Entry("changed kubeReserved CPU", &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "static", KubeReserved: map[string]string{"cpu": "100m"}}, &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "none", KubeReserved: map[string]string{"cpu": "200m"}}, true, true, BeNil()),
		Entry("changed kubeReserved memory", &kubeletconfigv1beta1.KubeletConfiguration{KubeReserved: map[string]string{"memory": "100Mi"}}, &kubeletconfigv1beta1.KubeletConfiguration{KubeReserved: map[string]string{"memory": "200Mi"}}, false, true, BeNil()),
		Entry("changed kubeReserved ephemeral-storage", &kubeletconfigv1beta1.KubeletConfiguration{KubeReserved: map[string]string{"ephemeral-storage": "100Gi"}}, &kubeletconfigv1beta1.KubeletConfiguration{KubeReserved: map[string]string{"ephemeral-storage": "200Gi"}}, false, true, BeNil()),
		Entry("changed kubeReserved PID", &kubeletconfigv1beta1.KubeletConfiguration{KubeReserved: map[string]string{"pids": "10k"}}, &kubeletconfigv1beta1.KubeletConfiguration{KubeReserved: map[string]string{"pids": "20k"}}, false, true, BeNil()),

		Entry("invalid systemReserved CPU", &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "static", KubeReserved: map[string]string{"cpu": "100m"}, SystemReserved: map[string]string{"cpu": "aoeu"}}, &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "none", KubeReserved: map[string]string{"cpu": "100m"}}, true, false, MatchError(ContainSubstring("failed to parse"))),
		Entry("changed systemReserved CPU", &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "static", SystemReserved: map[string]string{"cpu": "100m"}}, &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "none", SystemReserved: map[string]string{"cpu": "200m"}}, true, true, BeNil()),
		Entry("changed systemReserved memory", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"memory": "100Mi"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"memory": "200Mi"}}, false, true, BeNil()),
		Entry("changed systemReserved ephemeral-storage", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"ephemeral-storage": "100Gi"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"ephemeral-storage": "200Gi"}}, false, true, BeNil()),
		Entry("changed systemReserved PID", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"pids": "10k"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"pids": "20k"}}, false, true, BeNil()),

		Entry("sum of systemReserved and kubeReserved cpu changed", &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "static", SystemReserved: map[string]string{"cpu": "100m"}, KubeReserved: map[string]string{"cpu": "100m"}}, &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "none", SystemReserved: map[string]string{"cpu": "200m"}, KubeReserved: map[string]string{"cpu": "200m"}}, true, true, BeNil()),
		Entry("sum of systemReserved and kubeReserved cpu remains same", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"cpu": "100m"}, KubeReserved: map[string]string{"cpu": "50m"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"cpu": "50m"}, KubeReserved: map[string]string{"cpu": "100m"}}, false, false, BeNil()),
		Entry("sum of systemReserved and kubeReserved memory changed", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"memory": "100Mi"}, KubeReserved: map[string]string{"memory": "100Mi"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"memory": "200Mi"}, KubeReserved: map[string]string{"memory": "200Mi"}}, false, true, BeNil()),
		Entry("sum of systemReserved and kubeReserved memory remains same", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"memory": "100Mi"}, KubeReserved: map[string]string{"memory": "50Mi"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"memory": "50Mi"}, KubeReserved: map[string]string{"memory": "100Mi"}}, false, false, BeNil()),
		Entry("sum of systemReserved and kubeReserved ephemeral-storage changed", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"ephemeral-storage": "100Gi"}, KubeReserved: map[string]string{"ephemeral-storage": "100Gi"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"ephemeral-storage": "200Gi"}, KubeReserved: map[string]string{"ephemeral-storage": "200Gi"}}, false, true, BeNil()),
		Entry("sum of systemReserved and kubeReserved ephemeral-storage remains same", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"ephemeral-storage": "100Gi"}, KubeReserved: map[string]string{"ephemeral-storage": "50Gi"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"ephemeral-storage": "50Gi"}, KubeReserved: map[string]string{"ephemeral-storage": "100Gi"}}, false, false, BeNil()),
		Entry("sum of systemReserved and kubeReserved PID changed", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"pid": "10k"}, KubeReserved: map[string]string{"pid": "10k"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"pid": "20k"}, KubeReserved: map[string]string{"pid": "20k"}}, false, true, BeNil()),
		Entry("sum of systemReserved and kubeReserved PID remains same", &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"pid": "10k"}, KubeReserved: map[string]string{"pid": "5k"}}, &kubeletconfigv1beta1.KubeletConfiguration{SystemReserved: map[string]string{"pid": "5k"}, KubeReserved: map[string]string{"pid": "10k"}}, false, false, BeNil()),

		Entry("changed evictionHard memory.available", &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "static", EvictionHard: map[string]string{"memory.available": "100Mi"}}, &kubeletconfigv1beta1.KubeletConfiguration{CPUManagerPolicy: "none", EvictionHard: map[string]string{"memory.available": "200Mi"}}, true, true, BeNil()),
		Entry("changed evictionHard imagefs.available", &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"imagefs.available": "100Mi"}}, &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"imagefs.available": "200Mi"}}, false, true, BeNil()),
		Entry("changed evictionHard imagefs.inodesFree", &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"imagefs.inodesFree": "1k"}}, &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"imagefs.inodesFree": "2k"}}, false, true, BeNil()),
		Entry("changed evictionHard nodefs.available", &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"nodefs.available": "100Mi"}}, &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"nodefs.available": "200Mi"}}, false, true, BeNil()),
		Entry("changed evictionHard nodefs.inodesFree", &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"nodefs.inodesFree": "1k"}}, &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"nodefs.inodesFree": "2k"}}, false, true, BeNil()),
		Entry("some other field changed in evictionHard", &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"foo": "bar"}}, &kubeletconfigv1beta1.KubeletConfiguration{EvictionHard: map[string]string{"foo": "baz"}}, false, false, BeNil()),
	)
})
