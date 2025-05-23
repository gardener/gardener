// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package aggregate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/aggregate"
)

var _ = Describe("ScrapeConfigs", func() {
	Describe("#CentralScrapeConfigs", func() {
		It("should return the expected objects", func() {
			Expect(aggregate.CentralScrapeConfigs()).To(HaveExactElements(
				&monitoringv1alpha1.ScrapeConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "prometheus"},
					Spec: monitoringv1alpha1.ScrapeConfigSpec{
						HonorTimestamps: ptr.To(false),
						MetricsPath:     ptr.To("/federate"),
						Params: map[string][]string{
							"match[]": {
								`{__name__=~"metering:.+", __name__!~"metering:.+(over_time|_seconds|:this_month)"}`,
								`{__name__=~"seed:(.+):(.+)"}`,
								`{job="kube-state-metrics",namespace=~"garden|extension-.+"}`,
								`{job="kube-state-metrics",namespace=""}`,
								`{job="cadvisor",namespace=~"garden|extension-.+"}`,
								`{job="etcd-druid",namespace="garden"}`,
							},
						},
						KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
							Role:       "Service",
							Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{"garden"}},
						}},
						RelabelConfigs: []monitoringv1.RelabelConfig{
							{
								SourceLabels: []monitoringv1.LabelName{
									"__meta_kubernetes_service_name",
									"__meta_kubernetes_service_port_name",
								},
								Regex:  "prometheus-cache;web",
								Action: "keep",
							},
							{
								Action:      "replace",
								Replacement: ptr.To("prometheus"),
								TargetLabel: "job",
							},
						},
					},
				},
			))
		})
	})
})
