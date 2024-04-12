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
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter/garden"
)

var _ = Describe("ScrapeConfig", func() {
	Describe("#ScrapeConfig", func() {
		var (
			namespace               = "namespace"
			kubeAPIServerTargets    = []monitoringv1alpha1.Target{"target1", "target2"}
			gardenerDashboardTarget = monitoringv1alpha1.Target("target3")
		)

		It("should compute the scrape configs", func() {
			Expect(ScrapeConfig(namespace, kubeAPIServerTargets, gardenerDashboardTarget)).To(ContainElements(
				&monitoringv1alpha1.ScrapeConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "garden-blackbox-gardener-apiserver",
						Namespace: namespace,
						Labels:    map[string]string{"prometheus": "garden"},
					},
					Spec: monitoringv1alpha1.ScrapeConfigSpec{
						Params:      map[string][]string{"module": {"http_gardener_apiserver"}},
						MetricsPath: ptr.To("/probe"),
						StaticConfigs: []monitoringv1alpha1.StaticConfig{{
							Targets: []monitoringv1alpha1.Target{"https://gardener-apiserver.garden.svc/healthz"},
							Labels:  map[monitoringv1.LabelName]string{"purpose": "availability"},
						}},
						RelabelConfigs: []*monitoringv1.RelabelConfig{
							{
								SourceLabels: []monitoringv1.LabelName{"__address__"},
								Separator:    ptr.To(";"),
								Regex:        `(.*)`,
								TargetLabel:  "__param_target",
								Replacement:  `$1`,
								Action:       "replace",
							},
							{
								SourceLabels: []monitoringv1.LabelName{"__param_target"},
								Separator:    ptr.To(";"),
								Regex:        `(.*)`,
								TargetLabel:  "instance",
								Replacement:  `$1`,
								Action:       "replace",
							},
							{
								Separator:   ptr.To(";"),
								Regex:       `(.*)`,
								TargetLabel: "__address__",
								Replacement: "blackbox-exporter:9115",
								Action:      "replace",
							},
							{
								Action:      "replace",
								Replacement: "blackbox-gardener-apiserver",
								TargetLabel: "job",
							},
						},
						MetricRelabelConfigs: []*monitoringv1.RelabelConfig{{
							SourceLabels: []monitoringv1.LabelName{"__name__"},
							Action:       "keep",
							Regex:        `^(probe_success|probe_http_status_code|probe_http_duration_seconds)$`,
						}},
					},
				},
				&monitoringv1alpha1.ScrapeConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "garden-blackbox-apiserver",
						Namespace: namespace,
						Labels:    map[string]string{"prometheus": "garden"},
					},
					Spec: monitoringv1alpha1.ScrapeConfigSpec{
						Params:      map[string][]string{"module": {"http_kube_apiserver"}},
						MetricsPath: ptr.To("/probe"),
						StaticConfigs: []monitoringv1alpha1.StaticConfig{{
							Targets: kubeAPIServerTargets,
							Labels:  map[monitoringv1.LabelName]string{"purpose": "availability"},
						}},
						RelabelConfigs: []*monitoringv1.RelabelConfig{
							{
								SourceLabels: []monitoringv1.LabelName{"__address__"},
								Separator:    ptr.To(";"),
								Regex:        `https://api\..*`,
								TargetLabel:  "__param_module",
								Replacement:  "http_kube_apiserver_root_cas",
								Action:       "replace",
							},
							{
								SourceLabels: []monitoringv1.LabelName{"__address__"},
								Separator:    ptr.To(";"),
								Regex:        `(.*)`,
								TargetLabel:  "__param_target",
								Replacement:  `$1`,
								Action:       "replace",
							},
							{
								SourceLabels: []monitoringv1.LabelName{"__param_target"},
								Separator:    ptr.To(";"),
								Regex:        `(.*)`,
								TargetLabel:  "instance",
								Replacement:  `$1`,
								Action:       "replace",
							},
							{
								Separator:   ptr.To(";"),
								Regex:       `(.*)`,
								TargetLabel: "__address__",
								Replacement: "blackbox-exporter:9115",
								Action:      "replace",
							},
							{
								Action:      "replace",
								Replacement: "blackbox-apiserver",
								TargetLabel: "job",
							},
						},
						MetricRelabelConfigs: []*monitoringv1.RelabelConfig{{
							SourceLabels: []monitoringv1.LabelName{"__name__"},
							Action:       "keep",
							Regex:        `^(probe_success|probe_http_status_code|probe_http_duration_seconds)$`,
						}},
					},
				},
				&monitoringv1alpha1.ScrapeConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "garden-blackbox-dashboard",
						Namespace: namespace,
						Labels:    map[string]string{"prometheus": "garden"},
					},
					Spec: monitoringv1alpha1.ScrapeConfigSpec{
						Params:      map[string][]string{"module": {"http_gardener_dashboard"}},
						MetricsPath: ptr.To("/probe"),
						StaticConfigs: []monitoringv1alpha1.StaticConfig{{
							Targets: []monitoringv1alpha1.Target{gardenerDashboardTarget},
							Labels:  map[monitoringv1.LabelName]string{"purpose": "availability"},
						}},
						RelabelConfigs: []*monitoringv1.RelabelConfig{
							{
								SourceLabels: []monitoringv1.LabelName{"__address__"},
								Separator:    ptr.To(";"),
								Regex:        `(.*)`,
								TargetLabel:  "__param_target",
								Replacement:  `$1`,
								Action:       "replace",
							},
							{
								SourceLabels: []monitoringv1.LabelName{"__param_target"},
								Separator:    ptr.To(";"),
								Regex:        `(.*)`,
								TargetLabel:  "instance",
								Replacement:  `$1`,
								Action:       "replace",
							},
							{
								Separator:   ptr.To(";"),
								Regex:       `(.*)`,
								TargetLabel: "__address__",
								Replacement: "blackbox-exporter:9115",
								Action:      "replace",
							},
							{
								Action:      "replace",
								Replacement: "blackbox-dashboard",
								TargetLabel: "job",
							},
						},
						MetricRelabelConfigs: []*monitoringv1.RelabelConfig{{
							SourceLabels: []monitoringv1.LabelName{"__name__"},
							Action:       "keep",
							Regex:        `^(probe_success|probe_http_status_code|probe_http_duration_seconds)$`,
						}},
					},
				},
			))
		})
	})
})
