// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubescheduler_test

import (
	"path/filepath"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo/v2"

	. "github.com/gardener/gardener/pkg/component/kubescheduler"
	"github.com/gardener/gardener/pkg/component/test"
)

var _ = Describe("Monitoring", func() {
	It("should successfully test the scrape config", func() {
		kubeScheduler := New(nil, "", nil, semver.MustParse("1.26.0"), "", 0, nil, semver.MustParse("1.25.0"))

		test.ScrapeConfigs(kubeScheduler, expectedScrapeConfig)
	})

	It("should successfully test the alerting rules", func() {
		kubeScheduler := New(nil, "", nil, semver.MustParse("1.26.4"), "", 0, nil, semver.MustParse("1.25.0"))

		test.AlertingRulesWithPromtool(
			kubeScheduler,
			map[string]string{"kube-scheduler.rules.yaml": expectedAlertingRule},
			filepath.Join("testdata", "monitoring_alertingrules.yaml"),
		)
	})
})

const (
	expectedScrapeConfig = `job_name: kube-scheduler
scheme: https
tls_config:
  insecure_skip_verify: true
authorization:
  type: Bearer
  credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
honor_labels: false
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: []
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: kube-scheduler;metrics
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(scheduler_binding_duration_seconds_bucket|scheduler_e2e_scheduling_duration_seconds_bucket|scheduler_scheduling_algorithm_duration_seconds_bucket|rest_client_requests_total|process_max_fds|process_open_fds)$
`

	expectedAlertingRule = `groups:
- name: kube-scheduler.rules
  rules:
  - alert: KubeSchedulerDown
    expr: absent(up{job="kube-scheduler"} == 1)
    for: 15m
    labels:
      service: kube-scheduler
      severity: critical
      type: seed
      visibility: all
    annotations:
      description: New pods are not being assigned to nodes.
      summary: Kube Scheduler is down.

  ### Scheduling duration ###
  - record: cluster:scheduler_e2e_scheduling_duration_seconds:quantile
    expr: histogram_quantile(0.99, sum(scheduler_e2e_scheduling_duration_seconds_bucket) BY (le, cluster))
    labels:
      quantile: "0.99"
  - record: cluster:scheduler_e2e_scheduling_duration_seconds:quantile
    expr: histogram_quantile(0.9, sum(scheduler_e2e_scheduling_duration_seconds_bucket) BY (le, cluster))
    labels:
      quantile: "0.9"
  - record: cluster:scheduler_e2e_scheduling_duration_seconds:quantile
    expr: histogram_quantile(0.5, sum(scheduler_e2e_scheduling_duration_seconds_bucket) BY (le, cluster))
    labels:
      quantile: "0.5"
  - record: cluster:scheduler_scheduling_algorithm_duration_seconds:quantile
    expr: histogram_quantile(0.99, sum(scheduler_scheduling_algorithm_duration_seconds_bucket) BY (le, cluster))
    labels:
      quantile: "0.99"
  - record: cluster:scheduler_scheduling_algorithm_duration_seconds:quantile
    expr: histogram_quantile(0.9, sum(scheduler_scheduling_algorithm_duration_seconds_bucket) BY (le, cluster))
    labels:
      quantile: "0.9"
  - record: cluster:scheduler_scheduling_algorithm_duration_seconds:quantile
    expr: histogram_quantile(0.5, sum(scheduler_scheduling_algorithm_duration_seconds_bucket) BY (le, cluster))
    labels:
      quantile: "0.5"
  - record: cluster:scheduler_binding_duration_seconds:quantile
    expr: histogram_quantile(0.99, sum(scheduler_binding_duration_seconds_bucket) BY (le, cluster))
    labels:
      quantile: "0.99"
  - record: cluster:scheduler_binding_duration_seconds:quantile
    expr: histogram_quantile(0.9, sum(scheduler_binding_duration_seconds_bucket) BY (le, cluster))
    labels:
      quantile: "0.9"
  - record: cluster:scheduler_binding_duration_seconds:quantile
    expr: histogram_quantile(0.5, sum(scheduler_binding_duration_seconds_bucket) BY (le, cluster))
    labels:
      quantile: "0.5"
`
)
