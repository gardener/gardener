// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"fmt"
	"strconv"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	otelcomponent "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/opentelemetrycollector"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/features"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

// CentralScrapeConfigs returns the central ScrapeConfig resources for the shoot prometheus.
func CentralScrapeConfigs(namespace, clusterCASecretName string, isWorkerless bool) []*monitoringv1alpha1.ScrapeConfig {
	out := []*monitoringv1alpha1.ScrapeConfig{
		// We fetch kubelet metrics from seed's kube-system Prometheus and filter the metrics in shoot's namespace.
		kubeletSeedScrapeConfig(namespace),
		seedServiceEndpointsScrapeConfig(namespace),
		prometheusShootScrapeConfig(),
	}

	if !isWorkerless {
		// Only add cadvisor and kube-kubelet scrape configs if OTEL dataplane collector is not enabled.
		// Otherwise, it scrapes these targets already.
		// TODO(bobi-wan): merge with other flag occurence in an if/else statement
		if !features.DefaultFeatureGate.Enabled(features.OpenTelemetryDataplaneCollector) {
			out = append(out, cadvisorScrapeConfig(clusterCASecretName), kubeletScrapeConfig(clusterCASecretName))
		} else {
			out = append(out, opentelemetryCollectorDataplaneScrapeConfig(clusterCASecretName))
		}

		if features.DefaultFeatureGate.Enabled(features.OpenTelemetryCollector) {
			out = append(out, opentelemetryCollectorNodesScrapeConfig(clusterCASecretName))
		}
	}

	return out
}

func kubeletSeedScrapeConfig(namespace string) *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-kubelet-seed",
		},
		Spec: monitoringv1alpha1.ScrapeConfigSpec{
			HonorTimestamps: ptr.To(false),
			MetricsPath:     ptr.To("/federate"),
			KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
				Role:       monitoringv1alpha1.KubernetesRoleService,
				Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{"garden"}},
			}},
			Params: map[string][]string{
				"match[]": {
					`{__name__=~"metering:.+",namespace="` + namespace + `"}`,
					`{__name__=~"container_.+",job="cadvisor",namespace="` + namespace + `"}`,
					`{__name__=~"kube_.+",job="kube-state-metrics",namespace="` + namespace + `"}`,
					`{__name__=~"etcddruid_.+",job="etcd-druid",etcd_namespace="` + namespace + `"}`,
				},
			},
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
					Replacement: ptr.To("kube-kubelet-seed"),
					TargetLabel: "job",
				},
			},
			MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
				// "shoot-control-plane" references the namespace of the shoot control-plane pods in the seed
				Replacement: ptr.To("shoot-control-plane"),
				TargetLabel: "namespace",
			}},
		},
	}
}

func seedServiceEndpointsScrapeConfig(namespace string) *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "annotated-seed-service-endpoints",
		},
		Spec: monitoringv1alpha1.ScrapeConfigSpec{
			HonorLabels: ptr.To(false),
			KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
				Role:       monitoringv1alpha1.KubernetesRoleEndpoint,
				Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{namespace}},
			}},
			SampleLimit: ptr.To(uint64(500)),
			RelabelConfigs: []monitoringv1.RelabelConfig{
				// TODO(bobi-wan): use JobName here as well?
				{
					Action:      "replace",
					Replacement: ptr.To("annotated-seed-service-endpoints"),
					TargetLabel: "job",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_annotation_prometheus_io_scrape"},
					Action:       "keep",
					Regex:        `true`,
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_annotation_prometheus_io_scheme"},
					Action:       "replace",
					Regex:        `(https?)`,
					TargetLabel:  "__scheme__",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_annotation_prometheus_io_path"},
					Action:       "replace",
					Regex:        `(.+)`,
					TargetLabel:  "__metrics_path__",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__address__", "__meta_kubernetes_service_annotation_prometheus_io_port"},
					Action:       "replace",
					Regex:        `([^:]+)(?::\d+)?;(\d+)`,
					Replacement:  ptr.To("$1:$2"),
					TargetLabel:  "__address__",
				},
				{
					Action: "labelmap",
					Regex:  `__meta_kubernetes_service_label_(.+)`,
				},
				{
					// TODO(bobi-wan): aren't service_name labels populated
					// only with role="service" in service discovery? This
					// one uses role="endpoint" discovery.
					// Also, why relabel the "job" label a second time?
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_name"},
					Action:       "replace",
					TargetLabel:  "job",
				},
				{
					// TODO(bobi-wan): why relabel the "job" label a third time?
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_annotation_prometheus_io_name"},
					Action:       "replace",
					Regex:        `(.+)`,
					TargetLabel:  "job",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name"},
					TargetLabel:  "pod",
				},
			},
			MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
				SourceLabels: []monitoringv1.LabelName{"__name__"},
				Action:       "drop",
				Regex:        `^rest_client_request_latency_seconds.+$`,
			}},
		},
	}
}

func prometheusShootScrapeConfig() *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prometheus-" + Label,
		},
		Spec: monitoringv1alpha1.ScrapeConfigSpec{
			HonorLabels: ptr.To(false),
			StaticConfigs: []monitoringv1alpha1.StaticConfig{{
				Targets: []monitoringv1alpha1.Target{"localhost:9090"},
			}},
			RelabelConfigs: []monitoringv1.RelabelConfig{
				{
					Action:      "replace",
					Replacement: ptr.To("prometheus-" + Label),
					TargetLabel: "job",
				},
				{
					Action: "labelmap",
					Regex:  `__meta_kubernetes_service_label_(.+)`,
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name"},
					TargetLabel:  "pod",
				},
			},
		},
	}
}

func cadvisorScrapeConfig(clusterCASecretName string) *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cadvisor",
		},
		Spec: monitoringv1alpha1.ScrapeConfigSpec{
			HonorLabels:     ptr.To(false),
			HonorTimestamps: ptr.To(false),
			Scheme:          ptr.To(monitoringv1.SchemeHTTPS),
			Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: AccessSecretName},
				Key:                  resourcesv1alpha1.DataKeyToken,
			}},
			TLSConfig: &monitoringv1.SafeTLSConfig{CA: monitoringv1.SecretOrConfigMap{Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: clusterCASecretName},
				Key:                  secretsutils.DataKeyCertificateBundle,
			}}},
			KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
				// TODO(bobi-wan): why do we have a "node" role + a namespace configured?
				Role:            monitoringv1alpha1.KubernetesRoleNode,
				APIServer:       ptr.To("https://" + v1beta1constants.DeploymentNameKubeAPIServer + ":" + strconv.Itoa(kubeapiserverconstants.Port)),
				Namespaces:      &monitoringv1alpha1.NamespaceDiscovery{Names: []string{metav1.NamespaceSystem}},
				FollowRedirects: ptr.To(false),
				Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: AccessSecretName},
					Key:                  resourcesv1alpha1.DataKeyToken,
				}},
				TLSConfig: &monitoringv1.SafeTLSConfig{CA: monitoringv1.SecretOrConfigMap{Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: clusterCASecretName},
					Key:                  secretsutils.DataKeyCertificateBundle,
				}}},
			}},
			RelabelConfigs: []monitoringv1.RelabelConfig{
				// OK
				// Set in job_name already
				{
					Action:      "replace",
					Replacement: ptr.To("cadvisor"),
					TargetLabel: "job",
				},
				// OK
				{
					Action: "labelmap",
					Regex:  `__meta_kubernetes_node_label_(.+)`,
				},
				// OK - leaving default <host>:<ip>
				{
					TargetLabel: "__address__",
					Replacement: ptr.To(v1beta1constants.DeploymentNameKubeAPIServer + ":" + strconv.Itoa(kubeapiserverconstants.Port)),
				},
				// OK - leaving to /metrics/cadvisor
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_node_name"},
					Regex:        `(.+)`,
					Replacement:  ptr.To(`/api/v1/nodes/${1}/proxy/metrics/cadvisor`),
					TargetLabel:  "__metrics_path__",
				},
				// OK, added
				{
					TargetLabel: "type",
					Replacement: ptr.To("shoot"),
				},
			},
			MetricRelabelConfigs: []monitoringv1.RelabelConfig{
				// get system services
				// NOT OK, id still there, missing systemd_service_name
				{
					SourceLabels: []monitoringv1.LabelName{"id"},
					Action:       "replace",
					Regex:        `^/system\.slice/(.+)\.service$`,
					TargetLabel:  "systemd_service_name",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"id"},
					Action:       "replace",
					Regex:        `^/system\.slice/(.+)\.service$`,
					Replacement:  ptr.To(`$1`),
					TargetLabel:  "container",
				},
				monitoringutils.StandardMetricRelabelConfig(
					"container_cpu_cfs_periods_total",
					"container_cpu_cfs_throttled_seconds_total",
					"container_cpu_cfs_throttled_periods_total",
					"container_cpu_usage_seconds_total",
					"container_fs_inodes_total",
					"container_fs_limit_bytes",
					"container_fs_usage_bytes",
					"container_fs_reads_total",
					"container_fs_writes_total",
					"container_last_seen",
					"container_memory_working_set_bytes",
					"container_network_receive_bytes_total",
					"container_network_transmit_bytes_total",
				)[0],
				// We want to keep only metrics in kube-system namespace
				{
					SourceLabels: []monitoringv1.LabelName{"namespace"},
					Action:       "keep",
					// systemd containers don't have namespaces
					Regex: `(^$|^kube-system$)`,
				},
				// copied as is
				{
					SourceLabels: []monitoringv1.LabelName{"container", "__name__"},
					Action:       "drop",
					// The system container POD is used for networking
					Regex: `POD;(container_cpu_cfs_periods_total|container_cpu_cfs_throttled_seconds_total|container_cpu_cfs_throttled_periods_total|container_cpu_usage_seconds_total|container_fs_inodes_total|container_fs_limit_bytes|container_fs_usage_bytes|container_last_seen|container_memory_working_set_bytes)`,
				},
				// copied as is
				{
					SourceLabels: []monitoringv1.LabelName{"__name__", "container", "interface", "id"},
					Action:       "keep",
					Regex:        `container_network.+;;(eth0;/.+|(en.+|tunl0|eth0);/)|.+;.+;.*;.*`,
				},
				// copied as is
				{
					SourceLabels: []monitoringv1.LabelName{"__name__", "container", "interface"},
					Action:       "drop",
					Regex:        `container_network.+;POD;(.{5,}|tun0|en.+)`,
				},
				// copied as is
				{
					SourceLabels: []monitoringv1.LabelName{"__name__", "id"},
					Regex:        `container_network.+;/`,
					Replacement:  ptr.To("true"),
					TargetLabel:  "host_network",
				},
				{
					Regex:  `^id$`,
					Action: "labeldrop",
				},
			},
		},
	}
}

func kubeletScrapeConfig(clusterCASecretName string) *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-kubelet",
		},
		Spec: monitoringv1alpha1.ScrapeConfigSpec{
			HonorLabels: ptr.To(false),
			Scheme:      ptr.To(monitoringv1.SchemeHTTPS),
			Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: AccessSecretName},
				Key:                  resourcesv1alpha1.DataKeyToken,
			}},
			TLSConfig: &monitoringv1.SafeTLSConfig{CA: monitoringv1.SecretOrConfigMap{Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: clusterCASecretName},
				Key:                  secretsutils.DataKeyCertificateBundle,
			}}},
			KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
				Role:            monitoringv1alpha1.KubernetesRoleNode,
				APIServer:       ptr.To("https://" + v1beta1constants.DeploymentNameKubeAPIServer),
				FollowRedirects: ptr.To(true),
				Namespaces:      &monitoringv1alpha1.NamespaceDiscovery{Names: []string{metav1.NamespaceSystem}},
				Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: AccessSecretName},
					Key:                  resourcesv1alpha1.DataKeyToken,
				}},
				TLSConfig: &monitoringv1.SafeTLSConfig{CA: monitoringv1.SecretOrConfigMap{Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: clusterCASecretName},
					Key:                  secretsutils.DataKeyCertificateBundle,
				}}},
			}},
			RelabelConfigs: []monitoringv1.RelabelConfig{
				{
					Action:      "replace",
					Replacement: ptr.To("kube-kubelet"),
					TargetLabel: "job",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_node_address_InternalIP"},
					TargetLabel:  "instance",
				},
				{
					Action: "labelmap",
					Regex:  `__meta_kubernetes_node_label_(.+)`,
				},
				{
					TargetLabel: "__address__",
					Replacement: ptr.To(v1beta1constants.DeploymentNameKubeAPIServer + ":" + strconv.Itoa(kubeapiserverconstants.Port)),
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_node_name"},
					Regex:        `(.+)`,
					Replacement:  ptr.To(`/api/v1/nodes/${1}/proxy/metrics`),
					TargetLabel:  "__metrics_path__",
				},
				{
					TargetLabel: "type",
					Replacement: ptr.To("shoot"),
				},
			},
			MetricRelabelConfigs: append(monitoringutils.StandardMetricRelabelConfig(
				"kubelet_running_pods",
				"process_max_fds",
				"process_open_fds",
				"kubelet_volume_stats_available_bytes",
				"kubelet_volume_stats_capacity_bytes",
				"kubelet_volume_stats_used_bytes",
				"kubelet_image_pull_duration_seconds_bucket",
				"kubelet_image_pull_duration_seconds_sum",
				"kubelet_image_pull_duration_seconds_count",
			), monitoringv1.RelabelConfig{
				SourceLabels: []monitoringv1.LabelName{"namespace"},
				Action:       "keep",
				// Not all kubelet metrics have a namespace label. That's why we also need to match empty namespace (^$).
				Regex: `(^$|^` + metav1.NamespaceSystem + `$)`,
			}),
		},
	}
}

func opentelemetryCollectorNodesScrapeConfig(clusterCASecretName string) *monitoringv1alpha1.ScrapeConfig {
	nodeMetricsURL := fmt.Sprintf("/api/v1/nodes/${1}:%d/proxy/metrics", otelcomponent.MetricsPort)
	return &monitoringv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "opentelemetry-collector-nodes",
		},
		Spec: monitoringv1alpha1.ScrapeConfigSpec{
			HonorLabels: ptr.To(false),
			Scheme:      ptr.To(monitoringv1.SchemeHTTPS),
			Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: AccessSecretName},
				Key:                  resourcesv1alpha1.DataKeyToken,
			}},
			TLSConfig: &monitoringv1.SafeTLSConfig{CA: monitoringv1.SecretOrConfigMap{Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: clusterCASecretName},
				Key:                  secretsutils.DataKeyCertificateBundle,
			}}},
			KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
				Role:            monitoringv1alpha1.KubernetesRoleNode,
				APIServer:       ptr.To("https://" + v1beta1constants.DeploymentNameKubeAPIServer),
				FollowRedirects: ptr.To(true),
				// TODO(bobi-wan): role="node" + namespace filter again
				Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{metav1.NamespaceSystem}},
				Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: AccessSecretName},
					Key:                  resourcesv1alpha1.DataKeyToken,
				}},
				TLSConfig: &monitoringv1.SafeTLSConfig{CA: monitoringv1.SecretOrConfigMap{Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: clusterCASecretName},
					Key:                  secretsutils.DataKeyCertificateBundle,
				}}},
			}},
			RelabelConfigs: []monitoringv1.RelabelConfig{
				{
					// TODO(bobi-wan): again - static job name
					Action:      "replace",
					Replacement: ptr.To("opentelemetry-collector-nodes"),
					TargetLabel: "job",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_node_name"},
					TargetLabel:  "instance",
				},
				{
					Action: "labelmap",
					Regex:  `__meta_kubernetes_node_label_(.+)`,
				},
				{
					TargetLabel: "__address__",
					Replacement: ptr.To(v1beta1constants.DeploymentNameKubeAPIServer + ":" + strconv.Itoa(kubeapiserverconstants.Port)),
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_node_name"},
					Regex:        `(.+)`,
					Replacement:  ptr.To(nodeMetricsURL),
					TargetLabel:  "__metrics_path__",
				},
				{
					TargetLabel: "type",
					Replacement: ptr.To("shoot"),
				},
			},
			MetricRelabelConfigs: append(monitoringutils.StandardMetricRelabelConfig(
				"otelcol_exporter_.*",
				"otelcol_process_.*",
				"otelcol_receiver_.*",
				"otelcol_scraper_.*",
				"otelcol_processor_*",
			), monitoringv1.RelabelConfig{
				Action: "keep",
			}),
		},
	}
}

func opentelemetryCollectorDataplaneScrapeConfig(clusterCASecretName string) *monitoringv1alpha1.ScrapeConfig {
	podMetricsURL := "/api/v1/namespaces/kube-system/pods/${1}/proxy/metrics"
	return &monitoringv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "otel-collector-dataplane",
		},
		Spec: monitoringv1alpha1.ScrapeConfigSpec{
			JobName:     ptr.To("otel-collector-dataplane"),
			HonorLabels: ptr.To(true),
			Scheme:      ptr.To(monitoringv1.SchemeHTTPS),
			Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: AccessSecretName},
				Key:                  resourcesv1alpha1.DataKeyToken,
			}},
			TLSConfig: &monitoringv1.SafeTLSConfig{CA: monitoringv1.SecretOrConfigMap{Secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: clusterCASecretName},
				Key:                  secretsutils.DataKeyCertificateBundle,
			}}},
			KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
				Role:            monitoringv1alpha1.KubernetesRoleEndpoint,
				APIServer:       ptr.To("https://" + v1beta1constants.DeploymentNameKubeAPIServer),
				FollowRedirects: ptr.To(true),
				Namespaces:      &monitoringv1alpha1.NamespaceDiscovery{Names: []string{metav1.NamespaceSystem}},
				Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: AccessSecretName},
					Key:                  resourcesv1alpha1.DataKeyToken,
				}},
				TLSConfig: &monitoringv1.SafeTLSConfig{CA: monitoringv1.SecretOrConfigMap{Secret: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: clusterCASecretName},
					Key:                  secretsutils.DataKeyCertificateBundle,
				}}},
			}},
			RelabelConfigs: []monitoringv1.RelabelConfig{
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_name"},
					Action:       "keep",
					Regex:        "otel-collector-dataplane-deployment",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_endpoint_port_name"},
					Action:       "keep",
					Regex:        "metrics",
				},
				{
					TargetLabel: "__address__",
					Replacement: ptr.To(v1beta1constants.DeploymentNameKubeAPIServer + ":" + strconv.Itoa(kubeapiserverconstants.Port)),
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name"},
					Regex:        `(.+)`,
					TargetLabel:  "__metrics_path__",
					Replacement:  ptr.To(podMetricsURL),
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name"},
					TargetLabel:  "pod",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_node_name"},
					TargetLabel:  "instance",
				},
				{
					Action: "labelmap",
					Regex:  `__meta_kubernetes_pod_label_(.+)`,
				},
			},
		},
	}
}
