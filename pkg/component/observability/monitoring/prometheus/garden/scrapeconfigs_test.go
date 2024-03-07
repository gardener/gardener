// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garden_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
)

var _ = Describe("PrometheusRules", func() {
	Describe("#AdditionalScrapeConfigs", func() {
		It("should return the expected objects", func() {
			Expect(garden.AdditionalScrapeConfigs()).To(HaveExactElements(
				`job_name: cadvisor
honor_labels: false
scheme: https

tls_config:
  ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token

kubernetes_sd_configs:
- role: node

relabel_configs:
- source_labels: [__meta_kubernetes_node_address_InternalIP]
  target_label: instance
- action: labelmap
  regex: __meta_kubernetes_node_label_(.+)
- target_label: __address__
  replacement: kubernetes.default.svc
- source_labels: [__meta_kubernetes_node_name]
  regex: (.+)
  target_label: __metrics_path__
  replacement: /api/v1/nodes/${1}/proxy/metrics/cadvisor

metric_relabel_configs:
- source_labels: [ namespace ]
  regex: garden
  action: keep
- source_labels: [ __name__ ]
  regex: ^(container_cpu_usage_seconds_total|container_fs_reads_bytes_total|container_fs_writes_bytes_total|container_fs_inodes_total|container_fs_limit_bytes|container_fs_usage_bytes|container_last_seen|container_memory_working_set_bytes|container_network_receive_bytes_total|container_network_transmit_bytes_total)$
  action: keep
`,
			))
		})
	})
})
