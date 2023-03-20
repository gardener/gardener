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

package nodelocaldns_test

import (
	. "github.com/onsi/ginkgo/v2"

	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/nodelocaldns"
	"github.com/gardener/gardener/pkg/operation/botanist/component/test"
)

var _ = Describe("Monitoring", func() {
	var component component.MonitoringComponent

	BeforeEach(func() {
		component = New(nil, "", Values{})
	})

	It("should successfully test the scrape config", func() {
		test.ScrapeConfigs(component, expectedScrapeConfig, expectedErrorScrapeConfig)
	})
})

const (
	expectedScrapeConfig = `job_name: node-local-dns
scheme: https
tls_config:
  ca_file: /etc/prometheus/seed/ca.crt
authorization:
  type: Bearer
  credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
honor_labels: false
kubernetes_sd_configs:
- role: pod
  api_server: https://kube-apiserver:443
  tls_config:
    ca_file: /etc/prometheus/seed/ca.crt
  authorization:
    type: Bearer
    credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
relabel_configs:
- source_labels:
  - __meta_kubernetes_pod_name
  action: keep
  regex: node-local.*
- source_labels:
  - __meta_kubernetes_pod_container_name
  - __meta_kubernetes_pod_container_port_name
  action: keep
  regex: node-cache;metrics
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
- target_label: __address__
  replacement: kube-apiserver:443
- source_labels: [__meta_kubernetes_pod_name,__meta_kubernetes_pod_container_port_number]
  regex: (.+);(.+)
  target_label: __metrics_path__
  replacement: /api/v1/namespaces/kube-system/pods/${1}:${2}/proxy/metrics
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(coredns_build_info|coredns_cache_entries|coredns_cache_hits_total|coredns_cache_misses_total|coredns_dns_request_duration_seconds_count|coredns_dns_request_duration_seconds_bucket|coredns_dns_requests_total|coredns_dns_responses_total|coredns_forward_requests_total|coredns_forward_responses_total|coredns_kubernetes_dns_programming_duration_seconds_bucket|coredns_kubernetes_dns_programming_duration_seconds_count|coredns_kubernetes_dns_programming_duration_seconds_sum|process_max_fds|process_open_fds)$
`
	expectedErrorScrapeConfig = `job_name: node-local-dns-errors
scheme: https
tls_config:
  ca_file: /etc/prometheus/seed/ca.crt
authorization:
  type: Bearer
  credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
honor_labels: false
kubernetes_sd_configs:
- role: pod
  api_server: https://kube-apiserver:443
  tls_config:
    ca_file: /etc/prometheus/seed/ca.crt
  authorization:
    type: Bearer
    credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
relabel_configs:
- source_labels:
  - __meta_kubernetes_pod_name
  action: keep
  regex: node-local.*
- source_labels:
  - __meta_kubernetes_pod_container_name
  - __meta_kubernetes_pod_container_port_name
  action: keep
  regex: node-cache;errormetrics
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
- target_label: __address__
  replacement: kube-apiserver:443
- source_labels: [__meta_kubernetes_pod_name,__meta_kubernetes_pod_container_port_number]
  regex: (.+);(.+)
  target_label: __metrics_path__
  replacement: /api/v1/namespaces/kube-system/pods/${1}:${2}/proxy/metrics
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(coredns_nodecache_setup_errors_total)$
`
)
