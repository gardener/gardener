// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package apiserverproxy_test

import (
	. "github.com/onsi/ginkgo/v2"

	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/apiserverproxy"
	"github.com/gardener/gardener/pkg/component/test"
)

var _ = Describe("Monitoring", func() {
	var component component.MonitoringComponent

	BeforeEach(func() {
		component = New(nil, "", nil, Values{})
	})

	It("should successfully test the scrape config", func() {
		test.ScrapeConfigs(component, expectedScrapeConfig)
	})
})

const (
	expectedScrapeConfig = `job_name: apiserver-proxy
metrics_path: /metrics
scheme: https
tls_config:
  ca_file: /etc/prometheus/seed/ca.crt
authorization:
  type: Bearer
  credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
follow_redirects: false
kubernetes_sd_configs:
- role: endpoints
  api_server: https://kube-apiserver:443
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
  replacement: kube-apiserver:443
- source_labels: [__meta_kubernetes_pod_name, __meta_kubernetes_pod_container_port_number]
  regex: (.+);(.+)
  target_label: __metrics_path__
  replacement: /api/v1/namespaces/kube-system/pods/${1}:${2}/proxy/metrics
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(envoy_cluster_bind_errors|envoy_cluster_lb_healthy_panic|envoy_cluster_update_attempt|envoy_cluster_update_failure|envoy_cluster_upstream_cx_connect_ms_bucket|envoy_cluster_upstream_cx_length_ms_bucket|envoy_cluster_upstream_cx_none_healthy|envoy_cluster_upstream_cx_rx_bytes_total|envoy_cluster_upstream_cx_tx_bytes_total|envoy_listener_downstream_cx_destroy|envoy_listener_downstream_cx_length_ms_bucket|envoy_listener_downstream_cx_overflow|envoy_listener_downstream_cx_total|envoy_tcp_downstream_cx_no_route|envoy_tcp_downstream_cx_rx_bytes_total|envoy_tcp_downstream_cx_total|envoy_tcp_downstream_cx_tx_bytes_total)$
# we don't care about admin metrics
- source_labels: [ envoy_cluster_name ]
  regex: ^uds_admin$
  action: drop
- source_labels: [ envoy_listener_address ]
  regex: ^0.0.0.0_16910$
  action: drop
`
)
