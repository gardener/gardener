// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cluster_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter/shoot/cluster"
	componenttest "github.com/gardener/gardener/pkg/component/test"
)

var _ = Describe("PrometheusRule", func() {
	Describe("#PrometheusRule", func() {
		namespace := "namespace"

		It("should compute the scrape configs", func() {
			expectedPrometheusRule := &monitoringv1.PrometheusRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-blackbox-exporter-k8s-service-check",
					Namespace: namespace,
					Labels:    map[string]string{"prometheus": "shoot"},
				},
				Spec: monitoringv1.PrometheusRuleSpec{
					Groups: []monitoringv1.RuleGroup{{
						Name: "apiserver-connectivity-check.rules",
						Rules: []monitoringv1.Rule{
							{
								Alert: "ApiServerUnreachableViaKubernetesService",
								Expr:  intstr.FromString(`probe_success{job="blackbox-exporter-k8s-service-check"} == 0 or absent(probe_success{job="blackbox-exporter-k8s-service-check", instance="https://kubernetes.default.svc.cluster.local/healthz"})`),
								For:   ptr.To(monitoringv1.Duration("15m")),
								Labels: map[string]string{
									"service":    "apiserver-connectivity-check",
									"severity":   "critical",
									"type":       "shoot",
									"visibility": "all",
								},
								Annotations: map[string]string{
									"summary":     "Api server unreachable via the kubernetes service.",
									"description": "The Api server has been unreachable for 15 minutes via the kubernetes service in the shoot.",
								},
							},
							{
								Record: "shoot:availability",
								Expr:   intstr.FromString(`probe_success{job="blackbox-exporter-k8s-service-check"} == bool 1`),
								Labels: map[string]string{"kind": "shoot"},
							},
							{
								Record: "shoot:availability",
								Expr:   intstr.FromString(`probe_success{job="blackbox-apiserver"} == bool 1`),
								Labels: map[string]string{"kind": "seed"},
							},
							{
								Record: "shoot:availability",
								Expr:   intstr.FromString(`probe_success{job="tunnel-probe-apiserver-proxy"} == bool 1`),
								Labels: map[string]string{"kind": "vpn"},
							},
						},
					}},
				},
			}

			Expect(PrometheusRule(namespace)).To(ContainElements(expectedPrometheusRule))

			componenttest.PrometheusRule(expectedPrometheusRule, "testdata/shoot-blackbox-exporter-k8s-service-check.prometheusrule.test.yaml")
		})
	})
})
