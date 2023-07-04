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

package etcd_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/component/etcd"
)

var _ = Describe("Monitoring", func() {
	Describe("#CentralMonitoringConfiguration", func() {
		It("should return the expected scrape configs", func() {
			monitoringConfig, err := CentralMonitoringConfiguration()

			Expect(err).NotTo(HaveOccurred())
			Expect(monitoringConfig.ScrapeConfigs).To(ConsistOf(expectedCentralScrapeConfig))
			Expect(monitoringConfig.CAdvisorScrapeConfigMetricRelabelConfigs).To(BeEmpty())
		})
	})
})

const (
	expectedCentralScrapeConfig = `job_name: etcddruid
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [ garden ]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: etcd-druid;metrics
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(etcddruid_compaction_jobs_total|etcddruid_compaction_jobs_current|etcddruid_compaction_job_duration_seconds_bucket|etcddruid_compaction_job_duration_seconds_sum|etcddruid_compaction_job_duration_seconds_count|etcddruid_compaction_num_delta_events)$
`
)
