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

package kubecontrollermanager_test

import (
	"path/filepath"

	. "github.com/gardener/gardener/pkg/operation/botanist/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/operation/botanist/component/test"

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
			kubeControllerManager := New(nil, nil, "", semverVersion, "", nil, nil, nil, nil)
			test.ScrapeConfigs(kubeControllerManager, expectedScrapeConfig)
		},

		Entry("kubernetes 1.15", "1.15.5", expectedScrapeConfig),
		Entry("kubernetes 1.16", "1.16.6", expectedScrapeConfig),
		Entry("kubernetes 1.17", "1.17.7", expectedScrapeConfig),
		Entry("kubernetes 1.18", "1.18.8", expectedScrapeConfig),
		Entry("kubernetes 1.19", "1.19.9", expectedScrapeConfig),
	)

	It("should successfully test the alerting rules", func() {
		semverVersion, err := semver.NewVersion("1.18.4")
		Expect(err).NotTo(HaveOccurred())
		kubeControllerManager := New(nil, nil, "", semverVersion, "", nil, nil, nil, nil)

		test.AlertingRulesWithPromtool(
			kubeControllerManager,
			map[string]string{"kube-controller-manager.rules.yaml": expectedAlertingRule},
			filepath.Join("testdata", "monitoring_alertingrules.yaml"),
		)
	})
})

const (
	expectedScrapeConfig = `job_name: kube-controller-manager
scheme: https
tls_config:
  insecure_skip_verify: true
  cert_file: /etc/prometheus/seed/prometheus.crt
  key_file: /etc/prometheus/seed/prometheus.key
honor_labels: false
scrape_timeout: 15s
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: []
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: kube-controller-manager;metrics
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- source_labels: [ __name__ ]
  regex: ^(rest_client_requests_total|process_max_fds|process_open_fds)$
  action: keep
`

	expectedAlertingRule = `groups:
- name: kube-controller-manager.rules
  rules:
  - alert: KubeControllerManagerDown
    expr: absent(up{job="kube-controller-manager"} == 1)
    for: 15m
    labels:
      service: kube-controller-manager
      severity: critical
      type: seed
      visibility: all
    annotations:
      description: Deployments and replication controllers are not making progress.
      summary: Kube Controller Manager is down.`
)
