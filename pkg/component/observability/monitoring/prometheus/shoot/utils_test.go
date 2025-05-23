// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
)

var _ = Describe("Utils", func() {
	Describe("#ClusterComponentScrapeConfigSpec", func() {
		var (
			jobName = "job-name"
			metric1 = "metric-1"
			metric2 = "metric-2"
		)

		When("Kubernetes service discovery role is 'pod'", func() {
			var (
				podNamePrefix     = "pod-name-prefix"
				containerName     = "container-name"
				containerPortName = "container-port-name"
			)

			It("should generate the expected scrape config spec", func() {
				Expect(shoot.ClusterComponentScrapeConfigSpec(
					jobName,
					shoot.KubernetesServiceDiscoveryConfig{
						Role:              "Pod",
						PodNamePrefix:     podNamePrefix,
						ContainerName:     containerName,
						ContainerPortName: containerPortName,
					},
					metric1, metric2,
				)).To(Equal(monitoringv1alpha1.ScrapeConfigSpec{
					HonorLabels: ptr.To(false),
					Scheme:      ptr.To("HTTPS"),
					TLSConfig:   &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
					Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-shoot"},
						Key:                  "token",
					}},
					KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
						APIServer:  ptr.To("https://kube-apiserver"),
						Role:       "Pod",
						Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{"kube-system"}},
						Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-shoot"},
							Key:                  "token",
						}},
						TLSConfig: &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
					}},
					RelabelConfigs: []monitoringv1.RelabelConfig{
						{
							Action:      "replace",
							Replacement: ptr.To(jobName),
							TargetLabel: "job",
						},
						{
							TargetLabel: "type",
							Replacement: ptr.To("shoot"),
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name"},
							Action:       "keep",
							Regex:        podNamePrefix + ".*",
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_container_name", "__meta_kubernetes_pod_container_port_name"},
							Action:       "keep",
							Regex:        containerName + ";" + containerPortName,
						},
						{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_service_label_(.+)`,
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name"},
							TargetLabel:  "pod",
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_node_name"},
							TargetLabel:  "node",
						},
						{
							TargetLabel: "__address__",
							Replacement: ptr.To("kube-apiserver:443"),
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name", "__meta_kubernetes_pod_container_port_number"},
							Regex:        `(.+);(.+)`,
							TargetLabel:  "__metrics_path__",
							Replacement:  ptr.To("/api/v1/namespaces/kube-system/pods/${1}:${2}/proxy/metrics"),
						},
					},
					MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
						SourceLabels: []monitoringv1.LabelName{"__name__"},
						Action:       "keep",
						Regex:        `^(` + metric1 + `|` + metric2 + `)$`,
					}},
				}))
			})
		})

		When("Kubernetes service discovery role is 'endpoints'", func() {
			var (
				serviceName      = "service-name"
				endpointPortName = "endpoint-port-name"
			)

			It("should generate the expected scrape config spec", func() {
				Expect(shoot.ClusterComponentScrapeConfigSpec(
					jobName,
					shoot.KubernetesServiceDiscoveryConfig{
						Role:             "Endpoints",
						ServiceName:      serviceName,
						EndpointPortName: endpointPortName,
					},
					metric1, metric2,
				)).To(Equal(monitoringv1alpha1.ScrapeConfigSpec{
					HonorLabels: ptr.To(false),
					Scheme:      ptr.To("HTTPS"),
					TLSConfig:   &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
					Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-shoot"},
						Key:                  "token",
					}},
					KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
						APIServer:  ptr.To("https://kube-apiserver"),
						Role:       "Endpoints",
						Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{"kube-system"}},
						Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-shoot"},
							Key:                  "token",
						}},
						TLSConfig: &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
					}},
					RelabelConfigs: []monitoringv1.RelabelConfig{
						{
							Action:      "replace",
							Replacement: ptr.To(jobName),
							TargetLabel: "job",
						},
						{
							TargetLabel: "type",
							Replacement: ptr.To("shoot"),
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_name", "__meta_kubernetes_endpoint_port_name"},
							Action:       "keep",
							Regex:        serviceName + `;` + endpointPortName,
						},
						{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_service_label_(.+)`,
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name"},
							TargetLabel:  "pod",
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_node_name"},
							TargetLabel:  "node",
						},
						{
							TargetLabel: "__address__",
							Replacement: ptr.To("kube-apiserver:443"),
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name", "__meta_kubernetes_pod_container_port_number"},
							Regex:        `(.+);(.+)`,
							TargetLabel:  "__metrics_path__",
							Replacement:  ptr.To("/api/v1/namespaces/kube-system/pods/${1}:${2}/proxy/metrics"),
						},
					},
					MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
						SourceLabels: []monitoringv1.LabelName{"__name__"},
						Action:       "keep",
						Regex:        `^(` + metric1 + `|` + metric2 + `)$`,
					}},
				}))
			})
		})
	})
})
