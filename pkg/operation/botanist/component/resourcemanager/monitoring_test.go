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

package resourcemanager_test

import (
	. "github.com/onsi/ginkgo/v2"

	. "github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	"github.com/gardener/gardener/pkg/operation/botanist/component/test"
)

var _ = Describe("Monitoring", func() {
	resourceManager := New(nil, "", nil, "", Values{})

	Describe("#ScrapeConfig", func() {
		It("should successfully test the scrape configuration", func() {
			test.ScrapeConfigs(resourceManager, expectedScrapeConfig)
		})
	})
})

const (
	expectedScrapeConfig = `job_name: resourcemanager
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
  regex: gardener-resource-manager;metrics
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [ __meta_kubernetes_pod_name ]
  target_label: pod
metric_relabel_configs:
- source_labels: [ __name__ ]
  action: keep
  regex: ^(go_.+|workqueue_.+|controller_runtime_.+|rest_client_requests_total)$
`
)
