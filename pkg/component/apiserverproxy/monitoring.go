// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package apiserverproxy

import (
	"strconv"
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubeapiserver/constants"
)

const (
	monitoringPrometheusJobName = "apiserver-proxy"

	monitoringMetricEnvoyClusterBindErrors                  = "envoy_cluster_bind_errors"
	monitoringMetricEnvoyClusterLbHealthyPanic              = "envoy_cluster_lb_healthy_panic"
	monitoringMetricEnvoyClusterUpdateAttempt               = "envoy_cluster_update_attempt"
	monitoringMetricEnvoyClusterUpdateFailure               = "envoy_cluster_update_failure"
	monitoringMetricEnvoyClusterUpstreamCxConnectMsBucket   = "envoy_cluster_upstream_cx_connect_ms_bucket"
	monitoringMetricEnvoyClusterUpstreamCxLengthMsBucket    = "envoy_cluster_upstream_cx_length_ms_bucket"
	monitoringMetricEnvoyClusterUpstreamCxNone_healthy      = "envoy_cluster_upstream_cx_none_healthy"
	monitoringMetricEnvoyClusterUpstreamCxRxBytesTotal      = "envoy_cluster_upstream_cx_rx_bytes_total"
	monitoringMetricEnvoyClusterUpstreamCxTxBytesTotal      = "envoy_cluster_upstream_cx_tx_bytes_total"
	monitoringMetricEnvoyListenerDownstreamCxDestroy        = "envoy_listener_downstream_cx_destroy"
	monitoringMetricEnvoyListenerDownstreamCxLengthMsBucket = "envoy_listener_downstream_cx_length_ms_bucket"
	monitoringMetricEnvoyListenerDownstreamCxOverflow       = "envoy_listener_downstream_cx_overflow"
	monitoringMetricEnvoyListenerDownstreamCxTotal          = "envoy_listener_downstream_cx_total"
	monitoringMetricEnvoyTcpDownstreamCxNoRoute             = "envoy_tcp_downstream_cx_no_route"
	monitoringMetricEnvoyTcpDownstreamCxRxBytesTotal        = "envoy_tcp_downstream_cx_rx_bytes_total"
	monitoringMetricEnvoyTcpDownstreamCxTotal               = "envoy_tcp_downstream_cx_total"
	monitoringMetricEnvoyTcpDownstreamCxTxBytesTotal        = "envoy_tcp_downstream_cx_tx_bytes_total"
)

var (
	monitoringAllowedMetrics = []string{
		monitoringMetricEnvoyClusterBindErrors,
		monitoringMetricEnvoyClusterLbHealthyPanic,
		monitoringMetricEnvoyClusterUpdateAttempt,
		monitoringMetricEnvoyClusterUpdateFailure,
		monitoringMetricEnvoyClusterUpstreamCxConnectMsBucket,
		monitoringMetricEnvoyClusterUpstreamCxLengthMsBucket,
		monitoringMetricEnvoyClusterUpstreamCxNone_healthy,
		monitoringMetricEnvoyClusterUpstreamCxRxBytesTotal,
		monitoringMetricEnvoyClusterUpstreamCxTxBytesTotal,
		monitoringMetricEnvoyListenerDownstreamCxDestroy,
		monitoringMetricEnvoyListenerDownstreamCxLengthMsBucket,
		monitoringMetricEnvoyListenerDownstreamCxOverflow,
		monitoringMetricEnvoyListenerDownstreamCxTotal,
		monitoringMetricEnvoyTcpDownstreamCxNoRoute,
		monitoringMetricEnvoyTcpDownstreamCxRxBytesTotal,
		monitoringMetricEnvoyTcpDownstreamCxTotal,
		monitoringMetricEnvoyTcpDownstreamCxTxBytesTotal,
	}

	monitoringScrapeConfig = `job_name: ` + monitoringPrometheusJobName + `
metrics_path: /metrics
scheme: https
tls_config:
  ca_file: /etc/prometheus/seed/ca.crt
authorization:
  type: Bearer
  credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
kubernetes_sd_configs:
- role: endpoints
  api_server: https://` + v1beta1constants.DeploymentNameKubeAPIServer + `:` + strconv.Itoa(kubeapiserverconstants.Port) + `
  namespaces:
    names: [ kube-system ]
  tls_config:
    ca_file: /etc/prometheus/seed/ca.crt
  authorization:
    type: Bearer
    credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
relabel_configs:
- target_label: type
  replacement: shoot
- source_labels:
  - __meta_kubernetes_endpoints_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: apiserver-proxy;metrics
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
- source_labels: [ __meta_kubernetes_pod_node_name ]
  target_label: node
- target_label: __address__
  replacement: ` + v1beta1constants.DeploymentNameKubeAPIServer + `:` + strconv.Itoa(kubeapiserverconstants.Port) + `
- source_labels: [__meta_kubernetes_pod_name, __meta_kubernetes_pod_container_port_number]
  regex: (.+);(.+)
  target_label: __metrics_path__
  replacement: /api/v1/namespaces/kube-system/pods/${1}:${2}/proxy/metrics
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(` + strings.Join(monitoringAllowedMetrics, "|") + `)$
# we don't care about admin metrics
- source_labels: [ envoy_cluster_name ]
  regex: ^uds_admin$
  action: drop
- source_labels: [ envoy_listener_address ]
  regex: ^0.0.0.0_16910$
  action: drop
`
)

// ScrapeConfigs returns the scrape configurations for apiserver-proxy.
func (a *apiserverProxy) ScrapeConfigs() ([]string, error) {
	return []string{monitoringScrapeConfig}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (a *apiserverProxy) AlertingRules() (map[string]string, error) {
	return nil, nil
}
