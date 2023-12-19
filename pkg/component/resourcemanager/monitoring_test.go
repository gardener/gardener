// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package resourcemanager_test

import (
	. "github.com/onsi/ginkgo/v2"

	. "github.com/gardener/gardener/pkg/component/resourcemanager"
	"github.com/gardener/gardener/pkg/component/test"
)

var _ = Describe("Monitoring", func() {
	resourceManager := New(nil, "shoot--foo--bar", nil, Values{})

	Describe("#ScrapeConfig", func() {
		It("should successfully test the scrape configuration", func() {
			test.ScrapeConfigs(resourceManager, expectedScrapeConfig)
		})
	})
})

const (
	expectedScrapeConfig = `job_name: gardener-resource-manager
honor_labels: false
kubernetes_sd_configs:
- role: endpoints
  namespaces:
    names: [shoot--foo--bar]
relabel_configs:
- source_labels:
  - __meta_kubernetes_service_name
  - __meta_kubernetes_endpoint_port_name
  action: keep
  regex: gardener-resource-manager;metrics
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
- source_labels: [ __meta_kubernetes_namespace ]
  target_label: namespace
`
)
