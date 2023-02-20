// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vpnseedserver_test

import (
	. "github.com/onsi/ginkgo/v2"

	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/test"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/vpnseedserver"
)

var _ = Describe("Monitoring", func() {
	var vpnSeedServer component.MonitoringComponent

	BeforeEach(func() {
		vpnSeedServer = New(nil, "shoot--foo--bar", nil, nil, Values{})
	})

	It("should successfully test the scrape configs", func() {
		test.ScrapeConfigs(vpnSeedServer, expectedScrapeConfig)
	})
})

const (
	expectedScrapeConfig = `job_name: reversed-vpn-envoy-side-car
kubernetes_sd_configs:
- role: service
  namespaces:
    names: [shoot--foo--bar]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_service_port_name
  action: keep
  regex: vpn-seed-server;metrics
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(envoy_cluster_external_upstream_rq|envoy_cluster_external_upstream_rq_completed|envoy_cluster_external_upstream_rq_xx|envoy_cluster_lb_healthy_panic|envoy_cluster_original_dst_host_invalid|envoy_cluster_upstream_cx_active|envoy_cluster_upstream_cx_connect_attempts_exceeded|envoy_cluster_upstream_cx_connect_fail|envoy_cluster_upstream_cx_connect_timeout|envoy_cluster_upstream_cx_max_requests|envoy_cluster_upstream_cx_none_healthy|envoy_cluster_upstream_cx_overflow|envoy_cluster_upstream_cx_pool_overflow|envoy_cluster_upstream_cx_protocol_error|envoy_cluster_upstream_cx_rx_bytes_total|envoy_cluster_upstream_cx_total|envoy_cluster_upstream_cx_tx_bytes_total|envoy_cluster_upstream_rq|envoy_cluster_upstream_rq_completed|envoy_cluster_upstream_rq_max_duration_reached|envoy_cluster_upstream_rq_pending_overflow|envoy_cluster_upstream_rq_per_try_timeout|envoy_cluster_upstream_rq_retry|envoy_cluster_upstream_rq_retry_limit_exceeded|envoy_cluster_upstream_rq_retry_overflow|envoy_cluster_upstream_rq_rx_reset|envoy_cluster_upstream_rq_timeout|envoy_cluster_upstream_rq_total|envoy_cluster_upstream_rq_tx_reset|envoy_cluster_upstream_rq_xx|envoy_dns_cache_dynamic_forward_proxy_cache_config_dns_query_attempt|envoy_dns_cache_dynamic_forward_proxy_cache_config_dns_query_failure|envoy_dns_cache_dynamic_forward_proxy_cache_config_dns_query_success|envoy_dns_cache_dynamic_forward_proxy_cache_config_host_overflow|envoy_dns_cache_dynamic_forward_proxy_cache_config_num_hosts|envoy_http_downstream_cx_rx_bytes_total|envoy_http_downstream_cx_total|envoy_http_downstream_cx_tx_bytes_total|envoy_http_downstream_rq_xx|envoy_http_no_route|envoy_http_rq_total|envoy_listener_http_downstream_rq_xx|envoy_server_memory_allocated|envoy_server_memory_heap_size|envoy_server_memory_physical_size|envoy_cluster_upstream_cx_connect_ms_bucket|envoy_cluster_upstream_cx_connect_ms_sum|envoy_cluster_upstream_cx_length_ms_bucket|envoy_cluster_upstream_cx_length_ms_sum|envoy_http_downstream_cx_length_ms_bucket|envoy_http_downstream_cx_length_ms_sum)$
`
)
