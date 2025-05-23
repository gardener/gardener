// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"strconv"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	shootprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

// ScrapeConfig returns the scrape configs related to the blackbox-exporter for the shoot cluster use-case.
func ScrapeConfig(namespace string) []*monitoringv1alpha1.ScrapeConfig {
	return []*monitoringv1alpha1.ScrapeConfig{{
		ObjectMeta: monitoringutils.ConfigObjectMeta("blackbox-exporter-k8s-service-check", namespace, shootprometheus.Label),
		Spec: monitoringv1alpha1.ScrapeConfigSpec{
			HonorLabels: ptr.To(false),
			Scheme:      ptr.To("HTTPS"),
			Params: map[string][]string{
				"module": {moduleName},
				"target": {"https://kubernetes.default.svc.cluster.local/healthz"},
			},
			MetricsPath: ptr.To("/probe"),
			// This is needed because we do not fetch the correct cluster CA bundle right now
			TLSConfig: &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
			Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: shootprometheus.AccessSecretName},
				Key:                  resourcesv1alpha1.DataKeyToken,
			}},
			KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
				Role:       monitoringv1alpha1.KubernetesRoleService,
				Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{metav1.NamespaceSystem}},
				APIServer:  ptr.To("https://" + v1beta1constants.DeploymentNameKubeAPIServer + ":" + strconv.Itoa(kubeapiserverconstants.Port)),
				// This is needed because we do not fetch the correct cluster CA bundle right now
				TLSConfig: &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
				Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: shootprometheus.AccessSecretName},
					Key:                  resourcesv1alpha1.DataKeyToken,
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
					Replacement: ptr.To(v1beta1constants.DeploymentNameKubeAPIServer + ":" + strconv.Itoa(kubeapiserverconstants.Port)),
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
			MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
				"probe_duration_seconds",
				"probe_http_duration_seconds",
				"probe_success",
				"probe_http_status_code",
			),
		},
	}}
}
