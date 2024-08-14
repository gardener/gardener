// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kernelconfig

import (
	"fmt"
	"slices"
	"strconv"

	"k8s.io/component-helpers/node/util/sysctl"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
)

type component struct{}

// New returns a new kernel config component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "kernel-config"
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	var newData = make(map[string]string, len(data))

	for key, value := range data {
		newData[key] = value
	}

	if !ctx.KubeProxyEnabled {
		for key, value := range nonKubeProxyData {
			newData[key] = value
		}
	}

	if ptr.Deref(ctx.KubeletConfigParameters.ProtectKernelDefaults, true) {
		// Needed configuration by kubelet
		// The kubelet sets these values but it is not able to when protectKernelDefaults=true
		// Ref https://github.com/gardener/gardener/issues/7069
		newData[sysctl.VMOvercommitMemory] = strconv.Itoa(sysctl.VMOvercommitMemoryAlways)
		newData[sysctl.VMPanicOnOOM] = strconv.Itoa(sysctl.VMPanicOnOOMInvokeOOMKiller)
		newData[sysctl.KernelPanicOnOops] = strconv.Itoa(sysctl.KernelPanicOnOopsAlways)
		newData[sysctl.KernelPanic] = strconv.Itoa(sysctl.KernelPanicRebootTimeout)
		newData[sysctl.RootMaxKeys] = strconv.Itoa(sysctl.RootMaxKeysSetting)
		newData[sysctl.RootMaxBytes] = strconv.Itoa(sysctl.RootMaxBytesSetting)
	}

	// Custom kernel settings for worker group
	for key, value := range ctx.Sysctls {
		newData[key] = value
	}

	// Content should be in well-defined order
	keys := make([]string, 0, len(newData))
	for key := range newData {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	fileContent := ""
	for _, key := range keys {
		fileContent += fmt.Sprintf("%s = %s\n", key, newData[key])
	}

	kernelSettingsFile := extensionsv1alpha1.File{
		Path:        v1beta1constants.OperatingSystemConfigFilePathKernelSettings,
		Permissions: ptr.To[uint32](0644),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Data: fileContent,
			},
		},
	}

	systemdSysctlUnit := extensionsv1alpha1.Unit{
		// it needs to be reloaded, because the /etc/sysctl.d/ files are not present, when this is started for a first time
		Name:      "systemd-sysctl.service",
		Command:   ptr.To(extensionsv1alpha1.CommandRestart),
		Enable:    ptr.To(true),
		FilePaths: []string{kernelSettingsFile.Path},
	}

	return []extensionsv1alpha1.Unit{systemdSysctlUnit}, []extensionsv1alpha1.File{kernelSettingsFile}, nil
}

// Do not change the encoding here because extensions might modify it!
var data = map[string]string{
	// A higher vm.max_map_count is great for elasticsearch, mongo, or other mmap users
	// See https://github.com/kubernetes/kops/issues/1340
	"vm.max_map_count": "135217728",
	// See https://github.com/kubernetes/kubernetes/pull/38001
	"kernel.softlockup_panic":             "1",
	"kernel.softlockup_all_cpu_backtrace": "1",
	// See https://github.com/kubernetes/kube-deploy/issues/261
	// Increase the number of connections
	"net.core.somaxconn": "32768",
	// Maximum Socket Receive Buffer
	"net.core.rmem_max": "16777216",
	// Default Socket Send Buffer
	"net.core.wmem_max": "16777216",
	// explicitly enable IPv4 forwarding for all interfaces by default if not enabled by the OS image already
	"net.ipv4.conf.all.forwarding":     "1",
	"net.ipv4.conf.default.forwarding": "1",
	// explicitly enable IPv6 forwarding for all interfaces by default if not enabled by the OS image already
	"net.ipv6.conf.all.forwarding":     "1",
	"net.ipv6.conf.default.forwarding": "1",
	// enable martian packets
	"net.ipv4.conf.default.log_martians": "1",
	// Increase the maximum total buffer-space allocatable
	"net.ipv4.tcp_wmem": "4096 12582912 16777216",
	"net.ipv4.tcp_rmem": "4096 12582912 16777216",
	// Mitigate broken TCP connections
	// https://github.com/kubernetes/kubernetes/issues/41916#issuecomment-312428731
	"net.ipv4.tcp_retries2": "8",
	// Increase the number of outstanding syn requests allowed
	"net.ipv4.tcp_max_syn_backlog": "8096",
	// For persistent HTTP connections
	"net.ipv4.tcp_slow_start_after_idle": "0",
	// Increase the tcp-time-wait buckets pool size to prevent simple DOS attacks
	"net.ipv4.tcp_tw_reuse": "1",
	// Allowed local port range.
	"net.ipv4.ip_local_port_range": "32768 65535",
	// Max number of packets that can be queued on interface input
	// If kernel is receiving packets faster than can be processed
	// this queue increases
	"net.core.netdev_max_backlog": "16384",
	// Increase size of file handles and inode cache
	"fs.file-max": "20000000",
	// Max number of inotify instances and watches for a user
	// Since dockerd runs as a single user, the default instances value of 128 per user is too low
	// e.g. uses of inotify: nginx ingress controller, kubectl logs -f
	"fs.inotify.max_user_instances": "8192",
	"fs.inotify.max_user_watches":   "524288",
	// HANA requirement
	// See https://www.sap.com/developer/tutorials/hxe-ua-install-using-docker.html
	"fs.aio-max-nr":                "262144",
	"vm.memory_failure_early_kill": "1",
}

// Kube-proxy already sets the maximum conntrack size, but it may be useful for other scenarios.
var nonKubeProxyData = map[string]string{
	// A common problem on Linux systems is running out of space in the conntrack table,
	// which can cause poor iptables performance.
	// This can happen if you run a lot of workloads on a given host,
	// or if your workloads create a lot of TCP connections or bidirectional UDP streams.
	"net.netfilter.nf_conntrack_max": "1048576",
}
