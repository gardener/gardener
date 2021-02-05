// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package deployment_test

import (
	"path/filepath"

	"github.com/gardener/gardener/pkg/operation/botanist/component/test"
	. "github.com/gardener/gardener/pkg/operation/botanist/controlplane/kubeapiserver/deployment"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Monitoring", func() {

	DescribeTable("success tests for scrape config various kubernetes versions",
		func(version, expectedScrapeConfig string) {
			semverVersion, err := semver.NewVersion(version)
			Expect(err).NotTo(HaveOccurred())
			apiServer := New(
				nil,
				nil,
				nil,
				nil,
				semverVersion,
				seedNamespace,
				"",
				false,
				false,
				false,
				false,
				false,
				false,
				false,
				nil,
				nil,
				nil,
				0,
				0,
				nil,
				nil,
				APIServerSNIValues{},
				APIServerImages{},
			)
			test.ScrapeConfigs(apiServer, expectedScrapeConfig)
		},

		Entry("kubernetes 1.12", "1.12.2", expectedScrapeConfigK8sLess114),
		Entry("kubernetes 1.13", "1.13.3", expectedScrapeConfigK8sLess114),
		Entry("kubernetes 1.14", "1.14.4", expectedScrapeConfigK8sGreaterEqual114),
		Entry("kubernetes 1.15", "1.15.5", expectedScrapeConfigK8sGreaterEqual114),
		Entry("kubernetes 1.18", "1.18.8", expectedScrapeConfigK8sGreaterEqual114),
		Entry("kubernetes 1.19", "1.19.9", expectedScrapeConfigK8sGreaterEqual114),
		Entry("kubernetes 1.20", "1.20.9", expectedScrapeConfigK8sGreaterEqual114),
	)

	It("should successfully test the alerting rules", func() {
		apiServer := New(
			nil,
			nil,
			nil,
			nil,
			nil,
			seedNamespace,
			"",
			false,
			false,
			false,
			false,
			false,
			false,
			false,
			nil,
			nil,
			nil,
			0,
			0,
			nil,
			nil,
			APIServerSNIValues{},
			APIServerImages{},
		)

		test.AlertingRulesWithPromtool(
			apiServer,
			map[string]string{"kube-apiserver.rules.yaml": expectedAlertingRule},
			filepath.Join("testdata", "monitoring_alertingrules.yaml"),
		)
	})
})

const (
	seedNamespace                          = "shoot-ns"
	expectedScrapeConfigK8sGreaterEqual114 = `job_name: kube-apiserver
scheme: https
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [` + seedNamespace + `]
tls_config:
  insecure_skip_verify: true
  cert_file: /etc/prometheus/seed/prometheus.crt
  key_file: /etc/prometheus/seed/prometheus.key
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: kube-apiserver;kube-apiserver
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- source_labels: [ __name__ ]
  regex: ^(apiserver_audit_error_total|apiserver_audit_event_total|apiserver_current_inflight_requests|apiserver_current_inqueue_requests|apiserver_dropped_requests_total|apiserver_registered_watchers|apiserver_request_count|apiserver_request_duration_seconds_bucket|apiserver_request_terminations_total|apiserver_request_total|etcd_object_counts|process_max_fds|process_open_fds)$
  action: keep
`

	expectedScrapeConfigK8sLess114 = expectedScrapeConfigK8sGreaterEqual114 + `- source_labels: [ __name__ ]
  regex: ^apiserver_request_count$
  action: replace
  replacement: apiserver_request_total
  target_label: __name__
`

	expectedAlertingRule = `groups:
- name: kube-apiserver.rules
  rules:
  - alert: ApiServerNotReachable
    expr: probe_success{job="blackbox-apiserver"} == 0
    for: 5m
    labels:
      service: kube-apiserver
      severity: blocker
      type: seed
      visibility: all
    annotations:
      description: "API server not reachable via external endpoint: {{ $labels.instance }}."
      summary: API server not reachable (externally).
  - alert: KubeApiserverDown
    expr: absent(up{job="kube-apiserver"} == 1)
    for: 5m
    labels:
      service: kube-apiserver
      severity: blocker
      type: seed
      visibility: operator
    annotations:
      description: All API server replicas are down/unreachable, or all API server could not be found.
      summary: API server unreachable.
  - alert: KubeApiServerTooManyOpenFileDescriptors
    expr: 100 * process_open_fds{job="kube-apiserver"} / process_max_fds > 50
    for: 30m
    labels:
      service: kube-apiserver
      severity: warning
      type: seed
      visibility: owner
    annotations:
      description: 'The API server ({{ $labels.instance }}) is using {{ $value }}% of the available file/socket descriptors.'
      summary: 'The API server has too many open file descriptors'
  - alert: KubeApiServerTooManyOpenFileDescriptors
    expr: 100 * process_open_fds{job="kube-apiserver"} / process_max_fds{job="kube-apiserver"} > 80
    for: 30m
    labels:
      service: kube-apiserver
      severity: critical
      type: seed
      visibility: owner
    annotations:
      description: 'The API server ({{ $labels.instance }}) is using {{ $value }}% of the available file/socket descriptors.'
      summary: 'The API server has too many open file descriptors'
  # Some verbs excluded because they are expected to be long-lasting:
  # WATCHLIST is long-poll, CONNECT is "kubectl exec".
  - alert: KubeApiServerLatency
    expr: histogram_quantile(0.99, sum without (instance,resource) (rate(apiserver_request_duration_seconds_bucket{subresource!="log",verb!~"CONNECT|WATCHLIST|WATCH|PROXY proxy"}[5m]))) > 3
    for: 30m
    labels:
      service: kube-apiserver
      severity: warning
      type: seed
      visibility: owner
    annotations:
      description: Kube API server latency for verb {{ $labels.verb }} is high. This could be because the shoot workers and the control plane are in different regions. 99th percentile of request latency is greater than 3 seconds.
      summary: Kubernetes API server latency is high
  # TODO replace with better metrics in the future (wyb1)
  - record: shoot:apiserver_watch_duration:quantile
    expr: histogram_quantile(0.2, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.2"
  - record: shoot:apiserver_watch_duration:quantile
    expr: histogram_quantile(0.5, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.5"
  - record: shoot:apiserver_watch_duration:quantile
    expr: histogram_quantile(0.9, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.9"
  - record: shoot:apiserver_watch_duration:quantile
    expr: histogram_quantile(0.2, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.2"
  - record: shoot:apiserver_watch_duration:quantile
    expr: histogram_quantile(0.5, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.5"
  - record: shoot:apiserver_watch_duration:quantile
    expr: histogram_quantile(0.9, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))
    labels:
      quantile: "0.9"
  ### API auditlog ###
  - alert: KubeApiServerTooManyAuditlogFailures
    expr: sum(rate (apiserver_audit_error_total{plugin="webhook",job="kube-apiserver"}[5m])) / sum(rate(apiserver_audit_event_total{job="kube-apiserver"}[5m])) > bool 0.02 == 1
    for: 15m
    labels:
      service: auditlog
      severity: warning
      type: seed
      visibility: operator
    annotations:
      description: 'The API servers cumulative failure rate in logging audit events is greater than 2%.'
      summary: 'The kubernetes API server has too many failed attempts to log audit events'
  ### API latency ###
  - record: apiserver_latency_seconds:quantile
    expr: histogram_quantile(0.99, rate(apiserver_request_duration_seconds_bucket[5m]))
    labels:
      quantile: "0.99"
  - record: apiserver_latency:quantile
    expr: histogram_quantile(0.9, rate(apiserver_request_duration_seconds_bucket[5m]))
    labels:
      quantile: "0.9"
  - record: apiserver_latency_seconds:quantile
    expr: histogram_quantile(0.5, rate(apiserver_request_duration_seconds_bucket[5m]))
    labels:
      quantile: "0.5"

  - record: shoot:kube_apiserver:sum_by_pod
    expr: sum(up{job="kube-apiserver"}) by (pod)`
)
