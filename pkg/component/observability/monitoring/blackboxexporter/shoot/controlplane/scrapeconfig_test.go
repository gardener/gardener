// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter/shoot/controlplane"
)

var _ = Describe("ScrapeConfig", func() {
	Describe("#ScrapeConfig", func() {
		var (
			namespace           = "namespace"
			kubeAPIServerTarget = monitoringv1alpha1.Target("target1")
		)

		It("should compute the scrape configs", func() {
			Expect(ScrapeConfig(namespace, kubeAPIServerTarget)).To(ContainElements(
				&monitoringv1alpha1.ScrapeConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shoot-blackbox-apiserver",
						Namespace: namespace,
						Labels:    map[string]string{"prometheus": "shoot"},
					},
					Spec: monitoringv1alpha1.ScrapeConfigSpec{
						Params:      map[string][]string{"module": {"http_apiserver"}},
						MetricsPath: ptr.To("/probe"),
						StaticConfigs: []monitoringv1alpha1.StaticConfig{{
							Targets: []monitoringv1alpha1.Target{kubeAPIServerTarget},
						}},
						RelabelConfigs: []monitoringv1.RelabelConfig{
							{
								TargetLabel: "type",
								Replacement: ptr.To("seed"),
							},
							{
								SourceLabels: []monitoringv1.LabelName{"__address__"},
								TargetLabel:  "__param_target",
								Action:       "replace",
							},
							{
								SourceLabels: []monitoringv1.LabelName{"__param_target"},
								TargetLabel:  "instance",
								Action:       "replace",
							},
							{
								TargetLabel: "__address__",
								Replacement: ptr.To("blackbox-exporter:9115"),
								Action:      "replace",
							},
							{
								Action:      "replace",
								Replacement: ptr.To("blackbox-apiserver"),
								TargetLabel: "job",
							},
						},
					},
				},
			))
		})
	})
})
