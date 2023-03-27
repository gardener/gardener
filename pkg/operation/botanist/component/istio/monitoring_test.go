// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package istio_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/operation/botanist/component/istio"
)

var _ = Describe("Monitoring", func() {
	Describe("#AggregateMonitoringConfiguration", func() {
		It("should return the expected scrape configs", func() {
			monitoringConfig, err := AggregateMonitoringConfiguration()
			Expect(err).NotTo(HaveOccurred())
			Expect(monitoringConfig.ScrapeConfigs).To(ConsistOf(expectedScrapeConfigIstiod, expectedScrapeConfigIstioIngressGateway))
		})
	})
})

const (
	expectedScrapeConfigIstiod = `job_name: istiod
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [ istio-system ]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  - __meta_kubernetes_namespace
  action: keep
  regex: istiod;metrics;istio-system
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
- source_labels: [ __meta_kubernetes_namespace ]
  target_label: namespace
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(galley_validation_failed|galley_validation_passed|go_goroutines|go_memstats_alloc_bytes|go_memstats_heap_alloc_bytes|go_memstats_heap_inuse_bytes|go_memstats_heap_sys_bytes|go_memstats_stack_inuse_bytes|istio_build|pilot_conflict_inbound_listener|pilot_conflict_outbound_listener_http_over_current_tcp|pilot_conflict_outbound_listener_tcp_over_current_http|pilot_conflict_outbound_listener_tcp_over_current_tcp|pilot_k8s_cfg_events|pilot_proxy_convergence_time_bucket|pilot_services|pilot_total_xds_internal_errors|pilot_total_xds_rejects|pilot_virt_services|pilot_xds|pilot_xds_cds_reject|pilot_xds_eds_reject|pilot_xds_lds_reject|pilot_xds_push_context_errors|pilot_xds_pushes|pilot_xds_rds_reject|pilot_xds_write_timeout|process_cpu_seconds_total|process_open_fds|process_resident_memory_bytes|process_virtual_memory_bytes)$
`

	expectedScrapeConfigIstioIngressGateway = `job_name: istio-ingressgateway
metrics_path: /stats/prometheus
kubernetes_sd_configs:
- role: endpoints
  selectors:
  - role: "endpoints"
    field: "metadata.name=istio-ingressgateway"
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: istio-ingressgateway;tls-tunnel
- source_labels: [__meta_kubernetes_pod_ip]
  action: replace
  target_label: __address__
  regex: (.+)
  replacement: ${1}:15020
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
- source_labels: [ __meta_kubernetes_namespace ]
  target_label: namespace
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(envoy_cluster_upstream_cx_active|envoy_cluster_upstream_cx_connect_fail|envoy_cluster_upstream_cx_total|envoy_cluster_upstream_cx_tx_bytes_total|envoy_server_hot_restart_epoch|go_goroutines|go_memstats_alloc_bytes|go_memstats_heap_alloc_bytes|go_memstats_heap_inuse_bytes|go_memstats_heap_sys_bytes|go_memstats_stack_inuse_bytes|istio_build|istio_request_bytes_bucket|istio_request_bytes_sum|istio_request_duration_milliseconds_bucket|istio_request_duration_seconds_bucket|istio_requests_total|istio_response_bytes_bucket|istio_response_bytes_sum|istio_tcp_connections_closed_total|istio_tcp_connections_opened_total|istio_tcp_received_bytes_total|istio_tcp_sent_bytes_total|process_cpu_seconds_total|process_open_fds|process_resident_memory_bytes|process_virtual_memory_bytes)$
`
)
