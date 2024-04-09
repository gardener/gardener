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

package aggregate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/aggregate"
)

var _ = Describe("ServiceMonitors", func() {
	Describe("#CentralServiceMonitors", func() {
		It("should return the expected objects", func() {
			Expect(aggregate.CentralServiceMonitors()).To(HaveExactElements(&monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{Name: "shoot-prometheus"},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{MatchLabels: map[string]string{
						"app":  "prometheus",
						"role": "monitoring",
					}},
					NamespaceSelector: monitoringv1.NamespaceSelector{Any: true},
					Endpoints: []monitoringv1.Endpoint{{
						Path:            "/federate",
						HonorTimestamps: ptr.To(false),
						HonorLabels:     true,
						Params: map[string][]string{
							"match[]": {
								`{__name__="shoot:availability"}`,
								`{__name__=~"shoot:(.+):(.+)"}`,
								`{__name__="ALERTS"}`,
								`{__name__="prometheus_tsdb_lowest_timestamp"}`,
								`{__name__="prometheus_tsdb_storage_blocks_bytes"}`,
								`{__name__="kubeproxy_network_latency:quantile"}`,
								`{__name__="kubeproxy_sync_proxy:quantile"}`,
							},
						},
						Port: "metrics",
						RelabelConfigs: []*monitoringv1.RelabelConfig{
							{
								Action:      "replace",
								Replacement: "shoot-prometheus",
								TargetLabel: "job",
							},
							{
								SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_namespace"},
								Regex:        `shoot-(.+)`,
								Action:       "keep",
							},
						},
					}},
				},
			}))
		})
	})
})
