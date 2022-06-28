// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vpnseedserver

import (
	"bytes"
	"strings"
	"text/template"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const (
	monitoringPrometheusJobName = "reversed-vpn-envoy-side-car"

	monitoringMetricEnvoyClusterExternalUpstreamRq                             = "envoy_cluster_external_upstream_rq"
	monitoringMetricEnvoyClusterExternalUpstreamRqCompleted                    = "envoy_cluster_external_upstream_rq_completed"
	monitoringMetricEnvoyClusterExternalUpstreamRqXx                           = "envoy_cluster_external_upstream_rq_xx"
	monitoringMetricEnvoyClusterLbHealthyPanic                                 = "envoy_cluster_lb_healthy_panic"
	monitoringMetricEnvoyClusterOriginalDstHostInvalid                         = "envoy_cluster_original_dst_host_invalid"
	monitoringMetricEnvoyClusterUpstreamCxActive                               = "envoy_cluster_upstream_cx_active"
	monitoringMetricEnvoyClusterUpstreamCxConnectAttemptsExceeded              = "envoy_cluster_upstream_cx_connect_attempts_exceeded"
	monitoringMetricEnvoyClusterUpstreamCxConnectFail                          = "envoy_cluster_upstream_cx_connect_fail"
	monitoringMetricEnvoyClusterUpstreamCxConnectTimeout                       = "envoy_cluster_upstream_cx_connect_timeout"
	monitoringMetricEnvoyClusterUpstreamCxMaxRequests                          = "envoy_cluster_upstream_cx_max_requests"
	monitoringMetricEnvoyClusterUpstreamCxNoneHealthy                          = "envoy_cluster_upstream_cx_none_healthy"
	monitoringMetricEnvoyClusterUpstreamCxOverflow                             = "envoy_cluster_upstream_cx_overflow"
	monitoringMetricEnvoyClusterUpstreamCxPoolOverflow                         = "envoy_cluster_upstream_cx_pool_overflow"
	monitoringMetricEnvoyClusterUpstreamCxProtocolError                        = "envoy_cluster_upstream_cx_protocol_error"
	monitoringMetricEnvoyClusterUpstreamCxRxBytesTotal                         = "envoy_cluster_upstream_cx_rx_bytes_total"
	monitoringMetricEnvoyClusterUpstreamCxTotal                                = "envoy_cluster_upstream_cx_total"
	monitoringMetricEnvoyClusterUpstreamCxTxBytesTotal                         = "envoy_cluster_upstream_cx_tx_bytes_total"
	monitoringMetricEnvoyClusterUpstreamRq                                     = "envoy_cluster_upstream_rq"
	monitoringMetricEnvoyClusterUpstreamRqCompleted                            = "envoy_cluster_upstream_rq_completed"
	monitoringMetricEnvoyClusterUpstreamRqMaxDurationReached                   = "envoy_cluster_upstream_rq_max_duration_reached"
	monitoringMetricEnvoyClusterUpstreamRqPendingOverflow                      = "envoy_cluster_upstream_rq_pending_overflow"
	monitoringMetricEnvoyClusterUpstreamRqPerTryTimeout                        = "envoy_cluster_upstream_rq_per_try_timeout"
	monitoringMetricEnvoyClusterUpstreamRqRetry                                = "envoy_cluster_upstream_rq_retry"
	monitoringMetricEnvoyClusterUpstreamRqRetryLimitExceeded                   = "envoy_cluster_upstream_rq_retry_limit_exceeded"
	monitoringMetricEnvoyClusterUpstreamRqRetryOverflow                        = "envoy_cluster_upstream_rq_retry_overflow"
	monitoringMetricEnvoyClusterUpstreamRqRxReset                              = "envoy_cluster_upstream_rq_rx_reset"
	monitoringMetricEnvoyClusterUpstreamRqTimeout                              = "envoy_cluster_upstream_rq_timeout"
	monitoringMetricEnvoyClusterUpstreamRqTotal                                = "envoy_cluster_upstream_rq_total"
	monitoringMetricEnvoyClusterUpstreamRqTxReset                              = "envoy_cluster_upstream_rq_tx_reset"
	monitoringMetricEnvoyClusterUpstreamRqXx                                   = "envoy_cluster_upstream_rq_xx"
	monitoringMetricEnvoyDnsCacheDynamicForwardProxyCacheConfigDnsQueryAttempt = "envoy_dns_cache_dynamic_forward_proxy_cache_config_dns_query_attempt"
	monitoringMetricEnvoyDnsCacheDynamicForwardProxyCacheConfigDnsQueryFailure = "envoy_dns_cache_dynamic_forward_proxy_cache_config_dns_query_failure"
	monitoringMetricEnvoyDnsCacheDynamicForwardProxyCacheConfigDnsQuerySuccess = "envoy_dns_cache_dynamic_forward_proxy_cache_config_dns_query_success"
	monitoringMetricEnvoyDnsCacheDynamicForwardProxyCacheConfigHostOverflow    = "envoy_dns_cache_dynamic_forward_proxy_cache_config_host_overflow"
	monitoringMetricEnvoyDnsCacheDynamicForwardProxyCacheConfigNumHosts        = "envoy_dns_cache_dynamic_forward_proxy_cache_config_num_hosts"
	monitoringMetricEnvoyHttpDownstreamCxRxBytesTotal                          = "envoy_http_downstream_cx_rx_bytes_total"
	monitoringMetricEnvoyHttpDownstreamCxTotal                                 = "envoy_http_downstream_cx_total"
	monitoringMetricEnvoyHttpDownstreamCxTxBytesTotal                          = "envoy_http_downstream_cx_tx_bytes_total"
	monitoringMetricEnvoyHttpDownstreamRqXx                                    = "envoy_http_downstream_rq_xx"
	monitoringMetricEnvoyHttpNoRoute                                           = "envoy_http_no_route"
	monitoringMetricEnvoyHttpRqTotal                                           = "envoy_http_rq_total"
	monitoringMetricEnvoyListenerHttpDownstreamRqXx                            = "envoy_listener_http_downstream_rq_xx"
	monitoringMetricEnvoyServerMemoryAllocated                                 = "envoy_server_memory_allocated"
	monitoringMetricEnvoyServerMemoryHeapSize                                  = "envoy_server_memory_heap_size"
	monitoringMetricEnvoyServerMemoryPhysicalSize                              = "envoy_server_memory_physical_size"
	monitoringMetricEnvoyClusterUpstreamCxConnectMsBucket                      = "envoy_cluster_upstream_cx_connect_ms_bucket"
	monitoringMetricEnvoyClusterUpstreamCxConnectMsSum                         = "envoy_cluster_upstream_cx_connect_ms_sum"
	monitoringMetricEnvoyClusterUpstreamCxLengthMsBucket                       = "envoy_cluster_upstream_cx_length_ms_bucket"
	monitoringMetricEnvoyClusterUpstreamCxLengthMsSum                          = "envoy_cluster_upstream_cx_length_ms_sum"
	monitoringMetricEnvoyHttpDownstreamCxLengthMsBucket                        = "envoy_http_downstream_cx_length_ms_bucket"
	monitoringMetricEnvoyHttpDownstreamCxLengthMsSum                           = "envoy_http_downstream_cx_length_ms_sum"
)

var (
	monitoringAllowedMetrics = []string{
		monitoringMetricEnvoyClusterExternalUpstreamRq,
		monitoringMetricEnvoyClusterExternalUpstreamRqCompleted,
		monitoringMetricEnvoyClusterExternalUpstreamRqXx,
		monitoringMetricEnvoyClusterLbHealthyPanic,
		monitoringMetricEnvoyClusterOriginalDstHostInvalid,
		monitoringMetricEnvoyClusterUpstreamCxActive,
		monitoringMetricEnvoyClusterUpstreamCxConnectAttemptsExceeded,
		monitoringMetricEnvoyClusterUpstreamCxConnectFail,
		monitoringMetricEnvoyClusterUpstreamCxConnectTimeout,
		monitoringMetricEnvoyClusterUpstreamCxMaxRequests,
		monitoringMetricEnvoyClusterUpstreamCxNoneHealthy,
		monitoringMetricEnvoyClusterUpstreamCxOverflow,
		monitoringMetricEnvoyClusterUpstreamCxPoolOverflow,
		monitoringMetricEnvoyClusterUpstreamCxProtocolError,
		monitoringMetricEnvoyClusterUpstreamCxRxBytesTotal,
		monitoringMetricEnvoyClusterUpstreamCxTotal,
		monitoringMetricEnvoyClusterUpstreamCxTxBytesTotal,
		monitoringMetricEnvoyClusterUpstreamRq,
		monitoringMetricEnvoyClusterUpstreamRqCompleted,
		monitoringMetricEnvoyClusterUpstreamRqMaxDurationReached,
		monitoringMetricEnvoyClusterUpstreamRqPendingOverflow,
		monitoringMetricEnvoyClusterUpstreamRqPerTryTimeout,
		monitoringMetricEnvoyClusterUpstreamRqRetry,
		monitoringMetricEnvoyClusterUpstreamRqRetryLimitExceeded,
		monitoringMetricEnvoyClusterUpstreamRqRetryOverflow,
		monitoringMetricEnvoyClusterUpstreamRqRxReset,
		monitoringMetricEnvoyClusterUpstreamRqTimeout,
		monitoringMetricEnvoyClusterUpstreamRqTotal,
		monitoringMetricEnvoyClusterUpstreamRqTxReset,
		monitoringMetricEnvoyClusterUpstreamRqXx,
		monitoringMetricEnvoyDnsCacheDynamicForwardProxyCacheConfigDnsQueryAttempt,
		monitoringMetricEnvoyDnsCacheDynamicForwardProxyCacheConfigDnsQueryFailure,
		monitoringMetricEnvoyDnsCacheDynamicForwardProxyCacheConfigDnsQuerySuccess,
		monitoringMetricEnvoyDnsCacheDynamicForwardProxyCacheConfigHostOverflow,
		monitoringMetricEnvoyDnsCacheDynamicForwardProxyCacheConfigNumHosts,
		monitoringMetricEnvoyHttpDownstreamCxRxBytesTotal,
		monitoringMetricEnvoyHttpDownstreamCxTotal,
		monitoringMetricEnvoyHttpDownstreamCxTxBytesTotal,
		monitoringMetricEnvoyHttpDownstreamRqXx,
		monitoringMetricEnvoyHttpNoRoute,
		monitoringMetricEnvoyHttpRqTotal,
		monitoringMetricEnvoyListenerHttpDownstreamRqXx,
		monitoringMetricEnvoyServerMemoryAllocated,
		monitoringMetricEnvoyServerMemoryHeapSize,
		monitoringMetricEnvoyServerMemoryPhysicalSize,
		monitoringMetricEnvoyClusterUpstreamCxConnectMsBucket,
		monitoringMetricEnvoyClusterUpstreamCxConnectMsSum,
		monitoringMetricEnvoyClusterUpstreamCxLengthMsBucket,
		monitoringMetricEnvoyClusterUpstreamCxLengthMsSum,
		monitoringMetricEnvoyHttpDownstreamCxLengthMsBucket,
		monitoringMetricEnvoyHttpDownstreamCxLengthMsSum,
	}

	monitoringScrapeConfigTmpl = `job_name: ` + monitoringPrometheusJobName + `
kubernetes_sd_configs:
- role: service
  namespaces:
    names: [{{ .namespace }}]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_service_port_name
  action: keep
  regex: ` + ServiceName + `;` + envoyMetricsPortName + `
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(` + strings.Join(monitoringAllowedMetrics, "|") + `)$
`
	monitoringScrapeConfigTemplate *template.Template
)

func init() {
	var err error

	monitoringScrapeConfigTemplate, err = template.New("monitoring-scrape-config").Parse(monitoringScrapeConfigTmpl)
	utilruntime.Must(err)
}

// ScrapeConfigs returns the scrape configurations for Prometheus.
func (v *vpnSeedServer) ScrapeConfigs() ([]string, error) {
	var scrapeConfig bytes.Buffer

	if err := monitoringScrapeConfigTemplate.Execute(&scrapeConfig, map[string]interface{}{"namespace": v.namespace}); err != nil {
		return nil, err
	}

	return []string{scrapeConfig.String()}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (v *vpnSeedServer) AlertingRules() (map[string]string, error) {
	return nil, nil
}
