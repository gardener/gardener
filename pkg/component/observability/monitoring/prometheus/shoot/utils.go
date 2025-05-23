// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

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
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

// KubernetesServiceDiscoveryConfig is the configuration for the Kubernetes service discovery.
type KubernetesServiceDiscoveryConfig struct {
	Role monitoringv1alpha1.KubernetesRole

	// when 'Role' is 'pod'
	PodNamePrefix     string
	ContainerName     string
	ContainerPortName string

	// when 'Role' is 'endpoints'
	ServiceName      string
	EndpointPortName string
}

// ClusterComponentScrapeConfigSpec returns the standard spec for a scrape config for components running in the shoot
// cluster (in this case, the shoot's kube-apiserver is used as proxy to scrape the metrics).
func ClusterComponentScrapeConfigSpec(jobName string, sdConfig KubernetesServiceDiscoveryConfig, allowedMetrics ...string) monitoringv1alpha1.ScrapeConfigSpec {
	var relabelConfigs []monitoringv1.RelabelConfig
	switch sdConfig.Role {
	case monitoringv1alpha1.KubernetesRolePod:
		relabelConfigs = []monitoringv1.RelabelConfig{
			{
				SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name"},
				Action:       "keep",
				Regex:        sdConfig.PodNamePrefix + ".*",
			},
			{
				SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_container_name", "__meta_kubernetes_pod_container_port_name"},
				Action:       "keep",
				Regex:        sdConfig.ContainerName + ";" + sdConfig.ContainerPortName,
			},
		}

	case monitoringv1alpha1.KubernetesRoleEndpoint:
		relabelConfigs = []monitoringv1.RelabelConfig{{
			SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_name", "__meta_kubernetes_endpoint_port_name"},
			Action:       "keep",
			Regex:        sdConfig.ServiceName + `;` + sdConfig.EndpointPortName,
		}}
	}

	return monitoringv1alpha1.ScrapeConfigSpec{
		HonorLabels: ptr.To(false),
		Scheme:      ptr.To("HTTPS"),
		// This is needed because we do not fetch the correct cluster CA bundle right now
		TLSConfig: &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
		Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: AccessSecretName},
			Key:                  resourcesv1alpha1.DataKeyToken,
		}},
		KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
			Role:       sdConfig.Role,
			APIServer:  ptr.To("https://" + v1beta1constants.DeploymentNameKubeAPIServer),
			Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{metav1.NamespaceSystem}},
			Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: AccessSecretName},
				Key:                  resourcesv1alpha1.DataKeyToken,
			}},
			// This is needed because we do not fetch the correct cluster CA bundle right now
			TLSConfig: &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)},
		}},
		RelabelConfigs: append(
			append([]monitoringv1.RelabelConfig{
				{
					Action:      "replace",
					Replacement: ptr.To(jobName),
					TargetLabel: "job",
				},
				{
					TargetLabel: "type",
					Replacement: ptr.To("shoot"),
				},
			}, relabelConfigs...),
			monitoringv1.RelabelConfig{
				Action: "labelmap",
				Regex:  `__meta_kubernetes_service_label_(.+)`,
			},
			monitoringv1.RelabelConfig{
				SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name"},
				TargetLabel:  "pod",
			},
			monitoringv1.RelabelConfig{
				SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_node_name"},
				TargetLabel:  "node",
			},
			monitoringv1.RelabelConfig{
				TargetLabel: "__address__",
				Replacement: ptr.To(v1beta1constants.DeploymentNameKubeAPIServer + ":" + strconv.Itoa(kubeapiserverconstants.Port)),
			},
			monitoringv1.RelabelConfig{
				SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name", "__meta_kubernetes_pod_container_port_number"},
				Regex:        `(.+);(.+)`,
				TargetLabel:  "__metrics_path__",
				Replacement:  ptr.To("/api/v1/namespaces/" + metav1.NamespaceSystem + "/pods/${1}:${2}/proxy/metrics"),
			},
		),
		MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(allowedMetrics...),
	}
}
