// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

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
	expectedCentralScrapeConfig = `job_name: etcd-druid
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
