// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kernelconfig_test

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/component-helpers/node/util/sysctl"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kernelconfig"
)

var _ = Describe("Component", func() {
	var (
		component components.Component

		kubeletSysctlConfigComment = "#Needed configuration by kubelet\n" +
			"#The kubelet sets these values but it is not able to when protectKernelDefaults=true\n" +
			"#Ref https://github.com/gardener/gardener/issues/7069\n"
		kubeletSysctlConfig = kubeletSysctlConfigComment +
			fmt.Sprintf("%s = %s\n", sysctl.VMOvercommitMemory, strconv.Itoa(sysctl.VMOvercommitMemoryAlways)) +
			fmt.Sprintf("%s = %s\n", sysctl.VMPanicOnOOM, strconv.Itoa(sysctl.VMPanicOnOOMInvokeOOMKiller)) +
			fmt.Sprintf("%s = %s\n", sysctl.KernelPanicOnOops, strconv.Itoa(sysctl.KernelPanicOnOopsAlways)) +
			fmt.Sprintf("%s = %s\n", sysctl.KernelPanic, strconv.Itoa(sysctl.KernelPanicRebootTimeout)) +
			fmt.Sprintf("%s = %s\n", sysctl.RootMaxKeys, strconv.Itoa(sysctl.RootMaxKeysSetting)) +
			fmt.Sprintf("%s = %s\n", sysctl.RootMaxBytes, strconv.Itoa(sysctl.RootMaxBytesSetting))
		hardCodedKubeletSysctlConfig = kubeletSysctlConfigComment +
			"vm/overcommit_memory = 1\n" +
			"vm/panic_on_oom = 0\n" +
			"kernel/panic_on_oops = 1\n" +
			"kernel/panic = 10\n" +
			"kernel/keys/root_maxkeys = 1000000\n" +
			"kernel/keys/root_maxbytes = 25000000\n"
		customKernelSettingsComment = "#Custom kernel settings for worker group\n"
		dummySettingConfig          = customKernelSettingsComment + "my.kernel.setting = 123\n"
		dummySettingMap             = map[string]string{"my.kernel.setting": "123"}
	)

	BeforeEach(func() {
		component = New()
	})

	DescribeTable("#Config", func(k8sVersion, additionalData string, protectKernelDefaults *bool, sysctls map[string]string, kubeProxyEnabled bool) {
		units, files, err := component.Config(components.Context{
			KubernetesVersion: semver.MustParse(k8sVersion),
			KubeletConfigParameters: components.ConfigurableKubeletConfigParameters{
				ProtectKernelDefaults: protectKernelDefaults,
			},
			KubeProxyEnabled: kubeProxyEnabled,
			Sysctls:          sysctls,
		})
		unsortedData := data + additionalData
		linesWithComments := strings.Split(unsortedData, "\n")
		lines := []string{}
		for _, line := range linesWithComments {
			// Remove comments and empty lines
			if !strings.HasPrefix(line, "#") && line != "" {
				lines = append(lines, line)
			}
		}
		slices.Sort(lines)
		modifiedData := ""
		for _, line := range lines {
			modifiedData += line + "\n"
		}

		Expect(err).NotTo(HaveOccurred())

		systemdSysctlUnit := extensionsv1alpha1.Unit{
			Name:      "systemd-sysctl.service",
			Command:   ptr.To(extensionsv1alpha1.CommandRestart),
			Enable:    ptr.To(true),
			FilePaths: []string{"/etc/sysctl.d/99-k8s-general.conf"},
		}

		kernelSettingsFile := extensionsv1alpha1.File{
			Path:        "/etc/sysctl.d/99-k8s-general.conf",
			Permissions: ptr.To[uint32](0644),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Data: modifiedData,
				},
			},
		}

		Expect(units).To(ConsistOf(systemdSysctlUnit))
		Expect(files).To(ConsistOf(kernelSettingsFile))
	},
		Entry("should return the expected units and files when kubelet option protectKernelDefaults is set by default", "1.27.0", kubeletSysctlConfig, nil, nil, true),
		Entry("should return the expected units and files when kubelet option protectKernelDefaults is set to false", "1.27.0", "", ptr.To(false), nil, true),
		// This test prevents from unknowingly upgrading to a newer k8s version which may have different sysctl settings.
		Entry("should return the expected units and files if k8s version has not been upgraded", "1.27.0", hardCodedKubeletSysctlConfig, nil, nil, true),
		Entry("should return the expected units and files if configured to add kernel settings", "1.27.0", dummySettingConfig, ptr.To(false), dummySettingMap, true),
		Entry("should return the expected units and files if kube-proxy is disabled", "1.27.0", nonKubeProxyData, ptr.To(false), nil, false),
	)
})

const data = `# A higher vm.max_map_count is great for elasticsearch, mongo, or other mmap users
# See https://github.com/kubernetes/kops/issues/1340
vm.max_map_count = 135217728
# See https://github.com/kubernetes/kubernetes/pull/38001
kernel.softlockup_panic = 1
kernel.softlockup_all_cpu_backtrace = 1
# See https://github.com/kubernetes/kube-deploy/issues/261
# Increase the number of connections
net.core.somaxconn = 32768
# Maximum Socket Receive Buffer
net.core.rmem_max = 16777216
# Default Socket Send Buffer
net.core.wmem_max = 16777216
# explicitly enable IPv4 forwarding for all interfaces by default if not enabled by the OS image already
net.ipv4.conf.all.forwarding = 1
net.ipv4.conf.default.forwarding = 1
# explicitly enable IPv6 forwarding for all interfaces by default if not enabled by the OS image already
net.ipv6.conf.all.forwarding = 1
net.ipv6.conf.default.forwarding = 1
# enable martian packets
net.ipv4.conf.default.log_martians = 1
# Increase the maximum total buffer-space allocatable
net.ipv4.tcp_wmem = 4096 12582912 16777216
net.ipv4.tcp_rmem = 4096 12582912 16777216
# Mitigate broken TCP connections
# https://github.com/kubernetes/kubernetes/issues/41916#issuecomment-312428731
net.ipv4.tcp_retries2 = 8
# Increase the number of outstanding syn requests allowed
net.ipv4.tcp_max_syn_backlog = 8096
# For persistent HTTP connections
net.ipv4.tcp_slow_start_after_idle = 0
# Increase the tcp-time-wait buckets pool size to prevent simple DOS attacks
net.ipv4.tcp_tw_reuse = 1
# Allowed local port range.
net.ipv4.ip_local_port_range = 32768 65535
# Max number of packets that can be queued on interface input
# If kernel is receiving packets faster than can be processed
# this queue increases
net.core.netdev_max_backlog = 16384
# Increase size of file handles and inode cache
fs.file-max = 20000000
# Max number of inotify instances and watches for a user
# Since dockerd runs as a single user, the default instances value of 128 per user is too low
# e.g. uses of inotify: nginx ingress controller, kubectl logs -f
fs.inotify.max_user_instances = 8192
fs.inotify.max_user_watches = 524288
# HANA requirement
# See https://www.sap.com/developer/tutorials/hxe-ua-install-using-docker.html
fs.aio-max-nr = 262144
vm.memory_failure_early_kill = 1
`

const nonKubeProxyData = `# A common problem on Linux systems is running out of space in the conntrack table,
# which can cause poor iptables performance.
# This can happen if you run a lot of workloads on a given host,
# or if your workloads create a lot of TCP connections or bidirectional UDP streams.
net.netfilter.nf_conntrack_max = 1048576
`
