// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	_ "embed"
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
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

// CentralScrapeConfigs returns the central ScrapeConfig resources for the shoot prometheus.
func CentralScrapeConfigs(namespace, clusterCASecretName string, isWorkerless bool) []*monitoringv1alpha1.ScrapeConfig {
	out := []*monitoringv1alpha1.ScrapeConfig{
		// We fetch kubelet metrics from seed's kube-system Prometheus and filter the metrics in shoot's namespace.
		{
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
						`{job="cadvisor",namespace="` + namespace + `"}`,
						`{job="kube-state-metrics",namespace="` + namespace + `"}`,
						`{__name__=~"metering:.+",namespace="` + namespace + `"}`,
						`{job="etcd-druid",etcd_namespace="` + namespace + `"}`,
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
		},
		{
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
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_name"},
						Action:       "replace",
						TargetLabel:  "job",
					},
					{
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
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prometheus",
			},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				HonorLabels: ptr.To(false),
				StaticConfigs: []monitoringv1alpha1.StaticConfig{{
					Targets: []monitoringv1alpha1.Target{"localhost:9090"},
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{
					{
						Action:      "replace",
						Replacement: ptr.To("prometheus-shoot"),
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
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"process_max_fds",
					"process_open_fds",
					"process_resident_memory_bytes",
					"process_virtual_memory_bytes",
					"prometheus_config_last_reload_successful",
					"prometheus_engine_query_duration_seconds",
					"prometheus_rule_group_duration_seconds",
					"prometheus_rule_group_iterations_missed_total",
					"prometheus_rule_group_iterations_total",
					"prometheus_tsdb_blocks_loaded",
					"prometheus_tsdb_compactions_failed_total",
					"prometheus_tsdb_compactions_total",
					"prometheus_tsdb_compactions_triggered_total",
					"prometheus_tsdb_head_active_appenders",
					"prometheus_tsdb_head_chunks",
					"prometheus_tsdb_head_gc_duration_seconds",
					"prometheus_tsdb_head_gc_duration_seconds_count",
					"prometheus_tsdb_head_samples_appended_total",
					"prometheus_tsdb_head_series",
					"prometheus_tsdb_lowest_timestamp",
					"prometheus_tsdb_reloads_failures_total",
					"prometheus_tsdb_reloads_total",
					"prometheus_tsdb_storage_blocks_bytes",
					"prometheus_tsdb_wal_corruptions_total",
				),
			},
		},
	}

	if !isWorkerless {
		out = append(out,
			&monitoringv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cadvisor",
				},
				Spec: monitoringv1alpha1.ScrapeConfigSpec{
					HonorLabels:     ptr.To(false),
					HonorTimestamps: ptr.To(false),
					Scheme:          ptr.To("HTTPS"),
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
						{
							Action:      "replace",
							Replacement: ptr.To("cadvisor"),
							TargetLabel: "job",
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
							Replacement:  ptr.To(`/api/v1/nodes/${1}/proxy/metrics/cadvisor`),
							TargetLabel:  "__metrics_path__",
						},
						{
							TargetLabel: "type",
							Replacement: ptr.To("shoot"),
						},
					},
					MetricRelabelConfigs: []monitoringv1.RelabelConfig{
						// get system services
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
						{
							SourceLabels: []monitoringv1.LabelName{"container", "__name__"},
							Action:       "drop",
							// The system container POD is used for networking
							Regex: `POD;(container_cpu_cfs_periods_total|container_cpu_cfs_throttled_seconds_total|container_cpu_cfs_throttled_periods_total|container_cpu_usage_seconds_total|container_fs_inodes_total|container_fs_limit_bytes|container_fs_usage_bytes|container_last_seen|container_memory_working_set_bytes)`,
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__name__", "container", "interface", "id"},
							Action:       "keep",
							Regex:        `container_network.+;;(eth0;/.+|(en.+|tunl0|eth0);/)|.+;.+;.*;.*`,
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__name__", "container", "interface"},
							Action:       "drop",
							Regex:        `container_network.+;POD;(.{5,}|tun0|en.+)`,
						},
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
			},
			&monitoringv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kube-kubelet",
				},
				Spec: monitoringv1alpha1.ScrapeConfigSpec{
					HonorLabels: ptr.To(false),
					Scheme:      ptr.To("HTTPS"),
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
			},
		)
	}

	return out
}
