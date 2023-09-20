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

package kubeapiserver_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"

	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/component/test"
)

const testNS = "some-ns"

var _ = Describe("Monitoring", func() {
	var component component.MonitoringComponent

	BeforeEach(func() {
		component = New(nil, testNS, nil, Values{})
	})

	It("should successfully test the scrape config", func() {
		test.ScrapeConfigs(component, expectedScrapeConfig)
	})

	It("should successfully test the alerting rules", func() {
		test.AlertingRulesWithPromtool(
			component,
			map[string]string{"kube-apiserver.rules.yaml": expectedAlertingRule},
			filepath.Join("testdata", "monitoring_alertingrules.yaml"),
		)
	})
})

const (
	expectedScrapeConfig = `job_name: kube-apiserver
scheme: https
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [` + testNS + `]
tls_config:
  insecure_skip_verify: true
authorization:
  type: Bearer
  credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
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
  action: keep
  regex: ^(authentication_attempts|authenticated_user_requests|apiserver_admission_controller_admission_duration_seconds_.+|apiserver_admission_webhook_admission_duration_seconds_.+|apiserver_admission_step_admission_duration_seconds_.+|apiserver_admission_webhook_rejection_count|apiserver_audit_event_total|apiserver_audit_error_total|apiserver_audit_requests_rejected_total|apiserver_latency_seconds|apiserver_crd_webhook_conversion_duration_seconds_.+|apiserver_current_inflight_requests|apiserver_current_inqueue_requests|apiserver_longrunning_requests|apiserver_response_sizes_.+|apiserver_registered_watchers|apiserver_request_duration_seconds_.+|apiserver_request_terminations_total|apiserver_request_total|apiserver_request_count|apiserver_storage_transformation_duration_seconds_.+|apiserver_storage_transformation_operations_total|apiserver_init_events_total|apiserver_watch_events_sizes_.+|apiserver_watch_events_total|etcd_db_total_size_in_bytes|apiserver_storage_db_total_size_in_bytes|apiserver_storage_size_bytes|etcd_object_counts|apiserver_storage_objects|etcd_request_duration_seconds_.+|go_.+|process_max_fds|process_open_fds|watch_cache_capacity_increase_total|watch_cache_capacity_decrease_total|watch_cache_capacity|apiserver_cache_list_.+|apiserver_storage_list_.+)$
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
    expr: histogram_quantile(0.99, sum without (instance,resource,subresource) (rate(apiserver_request_duration_seconds_bucket{subresource!~"log|portforward|exec|proxy|attach",verb!~"CONNECT|WATCHLIST|WATCH|PROXY proxy"}[5m]))) > 3
    for: 30m
    labels:
      service: kube-apiserver
      severity: warning
      type: seed
      visibility: owner
    annotations:
      description: Kube API server latency for verb {{ $labels.verb }} is high. This could be because the shoot workers and the control plane are in different regions. 99th percentile of request latency is greater than 3 seconds.
      summary: Kubernetes API server latency is high
  # TODO(wyb1): replace with better metrics in the future
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
    expr: sum(rate (apiserver_audit_error_total{plugin!="log",job="kube-apiserver"}[5m])) / sum(rate(apiserver_audit_event_total{job="kube-apiserver"}[5m])) > bool 0.02 == 1
    for: 15m
    labels:
      service: auditlog
      severity: warning
      type: seed
      visibility: operator
    annotations:
      description: 'The API servers cumulative failure rate in logging audit events is greater than 2%.'
      summary: 'The kubernetes API server has too many failed attempts to log audit events'
  - record: shoot:apiserver_audit_event_total:sum
    expr: sum(rate(apiserver_audit_event_total{job="kube-apiserver"}[5m]))
  - record: shoot:apiserver_audit_error_total:sum
    expr: sum(rate(apiserver_audit_error_total{plugin!="log",job="kube-apiserver"}[5m]))
  ### API latency ###
  - record: apiserver_latency_seconds:quantile
    expr: histogram_quantile(0.99, sum without (instance, pod) (rate(apiserver_request_duration_seconds_bucket[5m])))
    labels:
      quantile: "0.99"
  - record: apiserver_latency_seconds:quantile
    expr: histogram_quantile(0.9, sum without (instance, pod) (rate(apiserver_request_duration_seconds_bucket[5m])))
    labels:
      quantile: "0.9"
  - record: apiserver_latency_seconds:quantile
    expr: histogram_quantile(0.5, sum without (instance, pod) (rate(apiserver_request_duration_seconds_bucket[5m])))
    labels:
      quantile: "0.5"

  - record: shoot:kube_apiserver:sum_by_pod
    expr: sum(up{job="kube-apiserver"}) by (pod)
  ### API failure rate ###
  - alert: ApiserverRequestsFailureRate
    expr: max(sum by(instance,resource,verb) (rate(apiserver_request_total{code=~"5.."}[10m])) / sum by(instance,resource,verb) (rate(apiserver_request_total[10m]))) * 100 > 10
    for: 30m
    labels:
      service: kube-apiserver
      severity: warning
      type: seed
      visibility: operator
    annotations:
      description: The API Server requests failure rate exceeds 10%.
      summary: Kubernetes API server failure rate is high
`
)
