// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/cache"
)

var _ = Describe("PrometheusRules", func() {
	Describe("#AdditionalScrapeConfigs", func() {
		When("seedIsShoot", func() {
			It("should return the expected objects  (with TLS verification skipped)", func() {
				result, err := cache.AdditionalScrapeConfigs(true)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveExactElements(
					`job_name: cadvisor
honor_timestamps: false
honor_labels: false
scheme: https
metrics_path: /metrics/cadvisor

tls_config:
  ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  insecure_skip_verify: true
bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token

kubernetes_sd_configs:
- role: node
relabel_configs:
- source_labels: [__meta_kubernetes_node_address_InternalIP]
  target_label: instance
- action: labelmap
  regex: __meta_kubernetes_node_label_(.+)
- target_label: type
  replacement: seed

metric_relabel_configs:
# get system services
- source_labels: [ id ]
  action: replace
  regex: ^/system\.slice/(.+)\.service$
  target_label: systemd_service_name
  replacement: '${1}'
- source_labels: [ id ]
  action: replace
  regex: ^/system\.slice/(.+)\.service$
  target_label: container
  replacement: '${1}'
- source_labels: [__name__]
  action: keep
  regex: ^(container_cpu_cfs_periods_total|container_cpu_cfs_throttled_periods_total|container_cpu_cfs_throttled_seconds_total|container_cpu_usage_seconds_total|container_fs_inodes_total|container_fs_limit_bytes|container_fs_reads_bytes_total|container_fs_usage_bytes|container_fs_writes_bytes_total|container_last_seen|container_memory_cache|container_memory_mapped_file|container_memory_rss|container_memory_usage_bytes|container_memory_working_set_bytes|container_network_receive_bytes_total|container_network_transmit_bytes_total|container_oom_events_total)$
- source_labels:
  - container
  - __name__
  # The system container POD is used for networking
  regex: POD;(container_cpu_cfs_periods_total|container_cpu_cfs_throttled_periods_total|container_cpu_cfs_throttled_seconds_total|container_cpu_usage_seconds_total|container_fs_inodes_total|container_fs_limit_bytes|container_fs_reads_bytes_total|container_fs_usage_bytes|container_fs_writes_bytes_total|container_last_seen|container_memory_cache|container_memory_mapped_file|container_memory_rss|container_memory_usage_bytes|container_memory_working_set_bytes|container_oom_events_total)
  action: drop
- source_labels: [ __name__, container, interface, id ]
  regex: container_network.+;;(eth0;/.+|(en.+|tunl0|eth0);/)|.+;.+;.*;.*
  action: keep
- source_labels: [ __name__, container, interface ]
  regex: container_network.+;POD;(.{5,}|tun0|en.+)
  action: drop
- source_labels: [ __name__, id ]
  regex: container_network.+;/
  target_label: host_network
  replacement: "true"
- source_labels: [ id ]
  regex: (/docker/.*)?/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-besteffort.slice/.*/docker/.*/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-besteffort.slice/.*
  action: drop
- regex: ^id$
  action: labeldrop
  # drop terraform pods
- source_labels: [ pod ]
  regex: ^.+\.tf-pod.+$
  action: drop
`,

					`job_name: kubelet
honor_labels: false
scheme: https

tls_config:
  ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  insecure_skip_verify: true
bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token

kubernetes_sd_configs:
- role: node
relabel_configs:
- source_labels: [__meta_kubernetes_node_address_InternalIP]
  target_label: instance
- action: labelmap
  regex: __meta_kubernetes_node_label_(.+)
- target_label: type
  replacement: seed

metric_relabel_configs:
- source_labels: [__name__]
  action: keep
  regex: ^(kubelet_volume_stats_available_bytes|kubelet_volume_stats_capacity_bytes|kubelet_volume_stats_used_bytes)$
`,
				))
			})
		})

		When("not a ManagedSeed", func() {
			It("should return the expected objects (with TLS verification enabled)", func() {
				result, err := cache.AdditionalScrapeConfigs(false)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveExactElements(
					`job_name: cadvisor
honor_timestamps: false
honor_labels: false
scheme: https
metrics_path: /metrics/cadvisor

tls_config:
  ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  insecure_skip_verify: false
bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token

kubernetes_sd_configs:
- role: node
relabel_configs:
- source_labels: [__meta_kubernetes_node_address_InternalIP]
  target_label: instance
- action: labelmap
  regex: __meta_kubernetes_node_label_(.+)
- target_label: type
  replacement: seed

metric_relabel_configs:
# get system services
- source_labels: [ id ]
  action: replace
  regex: ^/system\.slice/(.+)\.service$
  target_label: systemd_service_name
  replacement: '${1}'
- source_labels: [ id ]
  action: replace
  regex: ^/system\.slice/(.+)\.service$
  target_label: container
  replacement: '${1}'
- source_labels: [__name__]
  action: keep
  regex: ^(container_cpu_cfs_periods_total|container_cpu_cfs_throttled_periods_total|container_cpu_cfs_throttled_seconds_total|container_cpu_usage_seconds_total|container_fs_inodes_total|container_fs_limit_bytes|container_fs_reads_bytes_total|container_fs_usage_bytes|container_fs_writes_bytes_total|container_last_seen|container_memory_cache|container_memory_mapped_file|container_memory_rss|container_memory_usage_bytes|container_memory_working_set_bytes|container_network_receive_bytes_total|container_network_transmit_bytes_total|container_oom_events_total)$
- source_labels:
  - container
  - __name__
  # The system container POD is used for networking
  regex: POD;(container_cpu_cfs_periods_total|container_cpu_cfs_throttled_periods_total|container_cpu_cfs_throttled_seconds_total|container_cpu_usage_seconds_total|container_fs_inodes_total|container_fs_limit_bytes|container_fs_reads_bytes_total|container_fs_usage_bytes|container_fs_writes_bytes_total|container_last_seen|container_memory_cache|container_memory_mapped_file|container_memory_rss|container_memory_usage_bytes|container_memory_working_set_bytes|container_oom_events_total)
  action: drop
- source_labels: [ __name__, container, interface, id ]
  regex: container_network.+;;(eth0;/.+|(en.+|tunl0|eth0);/)|.+;.+;.*;.*
  action: keep
- source_labels: [ __name__, container, interface ]
  regex: container_network.+;POD;(.{5,}|tun0|en.+)
  action: drop
- source_labels: [ __name__, id ]
  regex: container_network.+;/
  target_label: host_network
  replacement: "true"
- source_labels: [ id ]
  regex: (/docker/.*)?/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-besteffort.slice/.*/docker/.*/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-besteffort.slice/.*
  action: drop
- regex: ^id$
  action: labeldrop
  # drop terraform pods
- source_labels: [ pod ]
  regex: ^.+\.tf-pod.+$
  action: drop
`,

					`job_name: kubelet
honor_labels: false
scheme: https

tls_config:
  ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  insecure_skip_verify: false
bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token

kubernetes_sd_configs:
- role: node
relabel_configs:
- source_labels: [__meta_kubernetes_node_address_InternalIP]
  target_label: instance
- action: labelmap
  regex: __meta_kubernetes_node_label_(.+)
- target_label: type
  replacement: seed

metric_relabel_configs:
- source_labels: [__name__]
  action: keep
  regex: ^(kubelet_volume_stats_available_bytes|kubelet_volume_stats_capacity_bytes|kubelet_volume_stats_used_bytes)$
`,
				))
			})
		})
	})
})
