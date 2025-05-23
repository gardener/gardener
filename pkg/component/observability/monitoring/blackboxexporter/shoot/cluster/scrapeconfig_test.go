// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cluster_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter/shoot/cluster"
)

var _ = Describe("ScrapeConfig", func() {
	Describe("#ScrapeConfig", func() {
		namespace := "namespace"

		It("should compute the scrape configs", func() {
			Expect(ScrapeConfig(namespace)).To(ContainElements(
				&monitoringv1alpha1.ScrapeConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shoot-blackbox-exporter-k8s-service-check",
						Namespace: namespace,
						Labels:    map[string]string{"prometheus": "shoot"},
					},
					Spec: monitoringv1alpha1.ScrapeConfigSpec{
						HonorLabels: ptr.To(false),
						Scheme:      ptr.To("HTTPS"),
						Params: map[string][]string{
							"module": {"http_kubernetes_service"},
							"target": {"https://kubernetes.default.svc.cluster.local/healthz"},
						},
						MetricsPath: ptr.To("/probe"),
						// This is needed because we do not fetch the correct cluster CA bundle right now
						TLSConfig: &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
						Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-shoot"},
							Key:                  "token",
						}},
						KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
							Role:       "Service",
							Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{"kube-system"}},
							APIServer:  ptr.To("https://kube-apiserver:443"),
							TLSConfig:  &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
							Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-shoot"},
								Key:                  "token",
							}},
						}},
						RelabelConfigs: []monitoringv1.RelabelConfig{
							{
								TargetLabel: "type",
								Replacement: ptr.To("shoot"),
							},
							{
								SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_name"},
								Action:       "keep",
								Regex:        "blackbox-exporter",
							},
							{
								TargetLabel: "__address__",
								Replacement: ptr.To("kube-apiserver:443"),
								Action:      "replace",
							},
							{
								SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_name"},
								Regex:        `(.+)`,
								TargetLabel:  "__metrics_path__",
								Replacement:  ptr.To(`/api/v1/namespaces/kube-system/services/${1}:probe/proxy/probe`),
							},
							{
								SourceLabels: []monitoringv1.LabelName{"__param_target"},
								TargetLabel:  "instance",
								Action:       "replace",
							},
							{
								Action:      "replace",
								Replacement: ptr.To("blackbox-exporter-k8s-service-check"),
								TargetLabel: "job",
							},
						},
						MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
							SourceLabels: []monitoringv1.LabelName{"__name__"},
							Action:       "keep",
							Regex:        `^(probe_duration_seconds|probe_http_duration_seconds|probe_success|probe_http_status_code)$`,
						}},
					},
				},
			))
		})
	})
})
