// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vpa_test

import (
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/test"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/vpa"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

	Context("Shoot Monitoring Configuration", func() {
		var vpa component.MonitoringComponent

		BeforeEach(func() {
			vpa = New(nil, "shoot--foo--bar", nil, Values{})
		})

		It("should successfully test the scrape configs", func() {
			test.ScrapeConfigs(vpa, expectedScrapeConfig)
		})
	})
})

const (
	expectedCentralScrapeConfig = `job_name: vpa-exporter
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
  regex: vpa-exporter;metrics;garden
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(vpa_status_recommendation|vpa_spec_container_resource_policy_allowed|vpa_metadata_generation)$
`
	expectedScrapeConfig = `job_name: vpa-exporter
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
  regex: vpa-exporter;metrics;garden
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(vpa_status_recommendation|vpa_spec_container_resource_policy_allowed|vpa_metadata_generation)$
- source_labels: [ namespace ]
  action: keep
  regex: ^shoot--foo--bar$
`
)
