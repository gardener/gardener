// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package hvpa_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/component/hvpa"
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
	expectedCentralScrapeConfig = `job_name: hvpa-controller
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [ garden ]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  - __meta_kubernetes_namespace
  action: keep
  regex: hvpa-controller;metrics;garden
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(hvpa_aggregate_applied_scaling_total|hvpa_aggregate_blocked_scalings_total|hvpa_spec_replicas|hvpa_status_replicas|hvpa_status_applied_hpa_current_replicas|hvpa_status_applied_hpa_desired_replicas|hvpa_status_applied_vpa_recommendation|hvpa_status_blocked_hpa_current_replicas|hvpa_status_blocked_hpa_desired_replicas|hvpa_status_blocked_vpa_recommendation)$
`
)
