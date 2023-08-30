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

package blackboxexporter_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"

	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/blackboxexporter"
	"github.com/gardener/gardener/pkg/component/test"
)

var _ = Describe("Monitoring", func() {
	var component component.MonitoringComponent

	BeforeEach(func() {
		component = New(nil, "", Values{})
	})

	It("should successfully test the scrape config", func() {
		test.ScrapeConfigs(component, expectedScrapeConfig)
	})

	It("should successfully test the alerting rules", func() {
		test.AlertingRulesWithPromtool(
			component,
			map[string]string{"apiserver-connectivity-check.rules.yaml": expectedAlertingRule},
			filepath.Join("testdata", "monitoring_alertingrules.yaml"),
		)
	})
})

const (
	expectedScrapeConfig = `job_name: blackbox-exporter-k8s-service-check
honor_labels: false
scheme: https
params:
  module:
  - http_kubernetes_service
  target:
  - https://kubernetes.default.svc.cluster.local/healthz
metrics_path: /probe
tls_config:
  ca_file: /etc/prometheus/seed/ca.crt
authorization:
  type: Bearer
  credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
follow_redirects: false
kubernetes_sd_configs:
- role: service
  namespaces:
    names: [ kube-system ]
  api_server: https://kube-apiserver:443
  tls_config:
    ca_file: /etc/prometheus/seed/ca.crt
  authorization:
    type: Bearer
    credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
relabel_configs:
- target_label: type
  replacement: shoot
- source_labels:
  - __meta_kubernetes_service_name
  action: keep
  regex: blackbox-exporter
- target_label: __address__
  replacement: kube-apiserver:443
- source_labels: [__meta_kubernetes_service_name]
  regex: (.+)
  target_label: __metrics_path__
  replacement: /api/v1/namespaces/kube-system/services/${1}:probe/proxy/probe
- source_labels: [ __param_target ]
  target_label: instance
  action: replace
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(probe_duration_seconds|probe_http_duration_seconds|probe_success|probe_http_status_code)$
`

	expectedAlertingRule = `groups:
- name: apiserver-connectivity-check.rules
  rules:
  - alert: ApiServerUnreachableViaKubernetesService
    expr: |
      probe_success{job="blackbox-exporter-k8s-service-check"} == 0
      or
      absent(probe_success{job="blackbox-exporter-k8s-service-check", instance="https://kubernetes.default.svc.cluster.local/healthz"})
    for: 15m
    labels:
      service: apiserver-connectivity-check
      severity: critical
      type: shoot
      visibility: all
    annotations:
      summary: Api server unreachable via the kubernetes service.
      description: The Api server has been unreachable for 15 minutes via the kubernetes service in the shoot.
  - record: shoot:availability
    expr: probe_success{job="blackbox-exporter-k8s-service-check"} == bool 1
    labels:
      kind: shoot
  - record: shoot:availability
    expr: probe_success{job="blackbox-apiserver"} == bool 1
    labels:
      kind: seed
  - record: shoot:availability
    expr: probe_success{job="tunnel-probe-apiserver-proxy"} == bool 1
    labels:
      kind: vpn
`
)
