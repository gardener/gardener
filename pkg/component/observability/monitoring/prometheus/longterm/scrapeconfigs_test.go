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

package longterm_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/longterm"
)

var _ = Describe("PrometheusRules", func() {
	Describe("#CentralScrapeConfigs", func() {
		It("should only contain the expected scrape configs", func() {
			Expect(longterm.CentralScrapeConfigs()).To(HaveExactElements(
				&monitoringv1alpha1.ScrapeConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "prometheus"},
					Spec: monitoringv1alpha1.ScrapeConfigSpec{
						StaticConfigs: []monitoringv1alpha1.StaticConfig{{
							Targets: []monitoringv1alpha1.Target{"localhost:9090"},
						}},
						RelabelConfigs: []*monitoringv1.RelabelConfig{{
							Action:      "replace",
							Replacement: "prometheus",
							TargetLabel: "job",
						}},
					},
				},
				&monitoringv1alpha1.ScrapeConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "cortex-frontend"},
					Spec: monitoringv1alpha1.ScrapeConfigSpec{
						StaticConfigs: []monitoringv1alpha1.StaticConfig{{
							Targets: []monitoringv1alpha1.Target{"localhost:9091"},
						}},
						RelabelConfigs: []*monitoringv1.RelabelConfig{{
							Action:      "replace",
							Replacement: "cortex-frontend",
							TargetLabel: "job",
						}},
					},
				},
				&monitoringv1alpha1.ScrapeConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "prometheus-garden"},
					Spec: monitoringv1alpha1.ScrapeConfigSpec{
						HonorLabels:     ptr.To(true),
						HonorTimestamps: ptr.To(true),
						MetricsPath:     ptr.To("/federate"),
						Params: map[string][]string{
							"match[]": {
								`{__name__="garden_shoot_info"}`,
								`{__name__=~"garden_shoot_info:timestamp:this_month"}`,
								`{__name__=~"metering:(cpu_requests|memory_requests|network|persistent_volume_claims|disk_usage_seconds|memory_usage_seconds).*:this_month"}`,
								`{__name__="garden_shoot_node_info"}`,
								`{__name__="garden_shoot_condition", condition="APIServerAvailable"}`,
							},
						},
						StaticConfigs: []monitoringv1alpha1.StaticConfig{{Targets: []monitoringv1alpha1.Target{"prometheus-garden"}}},
						RelabelConfigs: []*monitoringv1.RelabelConfig{{
							Action:      "replace",
							Replacement: "prometheus-garden",
							TargetLabel: "job",
						}},
					},
				},
			))
		})
	})
})
