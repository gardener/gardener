// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dataplanedeployment

import (
	"context"
	"time"

	"github.com/goccy/go-yaml"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"

	prometheusshoot "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"

	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	managedResourceName = "shoot-core-otel-collector-dataplane"
	componentName       = "otel-collector-dataplane-deployment"

	labelKeyComponent = "component"
	metricsPortName   = "metrics"
	portMetrics       = int32(8080)

	targetNamespace = metav1.NamespaceSystem
	// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or deleted.
	TimeoutWaitForManagedResource = 2 * time.Minute
)

// KubernetesScrapeConfig defines a Kubernetes service discovery scrape job.
type KubernetesScrapeConfig struct {
	JobName    string
	Role       string
	Namespaces []string
}

// Values is the values for OpenTelemetry Collector Dataplane Deployment configurations.
type Values struct {
	// Image is the container image used for the OTEL collector.
	Image string
	// Replicas is the number of replicas for the deployment.
	Replicas int32
	// KubernetesScrapeConfigs defines the Kubernetes service discovery scrape jobs.
	KubernetesScrapeConfigs []KubernetesScrapeConfig
}

type dataplaneDeployment struct {
	client    client.Client
	namespace string
	values    Values
}

// New creates a new instance of DeployWaiter for OpenTelemetry Collector Dataplane Deployment.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &dataplaneDeployment{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

func (d *dataplaneDeployment) Deploy(ctx context.Context) error {
	data, err := d.computeResourcesData()
	if err != nil {
		return err
	}
	return managedresources.CreateForShoot(ctx, d.client, d.namespace, managedResourceName, managedresources.LabelValueGardener, false, data)
}

func (d *dataplaneDeployment) Destroy(ctx context.Context) error {
	err := kubernetesutils.DeleteObjects(ctx, d.client, d.emptyScrapeConfig())
	if err != nil {
		return err
	}

	return managedresources.DeleteForShoot(ctx, d.client, d.namespace, managedResourceName)
}

func (d *dataplaneDeployment) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, d.client, d.namespace, managedResourceName)
}

func (d *dataplaneDeployment) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, d.client, d.namespace, managedResourceName)
}

func (d *dataplaneDeployment) emptyScrapeConfig() *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{ObjectMeta: monitoringutils.ConfigObjectMeta(componentName, d.namespace, prometheusshoot.Label)}
}

func (d *dataplaneDeployment) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      componentName,
				Namespace: targetNamespace,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: ptr.To(true),
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   componentName,
				Labels: getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"nodes", "services", "endpoints", "pods"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"nodes/metrics"},
					Verbs:     []string{"get"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   componentName,
				Labels: getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      serviceAccount.Name,
					Namespace: serviceAccount.Namespace,
				},
			},
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      componentName,
				Namespace: targetNamespace,
				Labels:    getLabels(),
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     metricsPortName,
						Port:     portMetrics,
						Protocol: corev1.ProtocolTCP,
					},
				},
				Selector: getLabels(),
			},
		}

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      componentName,
				Namespace: targetNamespace,
				Labels:    getLabels(),
			},
			Data: map[string]string{
				"config.yaml": d.getOTelConfig(),
			},
		}

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      componentName,
				Namespace: targetNamespace,
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					// TODO(Bobi-Wan): should it be GardenRoleMonitoring or GardenRoleObservability here?
					v1beta1constants.GardenRole:     v1beta1constants.GardenRoleMonitoring,
					managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
				}),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             &d.values.Replicas,
				RevisionHistoryLimit: ptr.To[int32](2),
				Selector: &metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: utils.MergeStringMaps(getLabels(), map[string]string{
							v1beta1constants.GardenRole:                            v1beta1constants.GardenRoleMonitoring,
							managedresources.LabelKeyOrigin:                        managedresources.LabelValueGardener,
							v1beta1constants.LabelNetworkPolicyShootFromSeed:       v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyShootToAPIServer:    v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyShootToKubelet:      v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToDNS:               v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyShootToNodeExporter: v1beta1constants.LabelNetworkPolicyAllowed,
						}),
					},
					Spec: corev1.PodSpec{
						PriorityClassName:  v1beta1constants.PriorityClassNameShootSystem700,
						ServiceAccountName: serviceAccount.Name,
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: ptr.To(true),
							RunAsUser:    ptr.To[int64](65534),
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Containers: []corev1.Container{
							{
								Name:            componentName,
								Image:           d.values.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/bin/otelcol",
									"--config=/etc/otel-collector/config.yaml",
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          metricsPortName,
										ContainerPort: portMetrics,
										Protocol:      corev1.ProtocolTCP,
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
									ReadOnlyRootFilesystem:   ptr.To(true),
									Capabilities: &corev1.Capabilities{
										Drop: []corev1.Capability{"ALL"},
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "config",
										MountPath: "/etc/otel-collector",
										ReadOnly:  true,
									},
									// {
									// 	Name:      "serviceaccount-token",
									// 	MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
									// 	ReadOnly:  true,
									// },
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: configMap.Name,
										},
									},
								},
							},
						},
					},
				},
			},
		}
	)

	return registry.AddAllAndSerialize(
		serviceAccount,
		clusterRole,
		clusterRoleBinding,
		service,
		configMap,
		deployment,
	)
}

func (d *dataplaneDeployment) getOTelConfig() string {
	config := map[string]any{
		"receivers": map[string]any{
			"prometheus": map[string]any{
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name":          "kube-kubelet",
							"honor_labels":      true,
							"scrape_interval":   "30s",
							"scheme":            "https",
							"bearer_token_file": "/var/run/secrets/kubernetes.io/serviceaccount/token",
							"tls_config": map[string]any{
								// "ca_file": "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
								"insecure_skip_verify": true,
							},
							"metrics_path": "/metrics",
							"kubernetes_sd_configs": []any{
								map[string]any{"role": "node"},
							},
							"relabel_configs": []any{
								map[string]any{
									"source_labels": []string{"__meta_kubernetes_node_name"},
									"action":        "replace",
									"target_label":  "instance",
								},
								// TODO(Bobi-Wan): check if needed
								map[string]any{
									"source_labels": []string{"__meta_kubernetes_node_name"},
									"action":        "replace",
									"target_label":  "node",
								},
							},
							"metric_relabel_configs": []any{
								map[string]any{
									"source_labels": []string{"__name__"},
									"action":        "keep",
									// TODO(Bobi-Wan): check if "up" is required
									"regex": "^(up|kubelet_running_pods|process_max_fds|process_open_fds|kubelet_volume_stats_available_bytes|kubelet_volume_stats_capacity_bytes|kubelet_volume_stats_used_bytes|kubelet_image_pull_duration_seconds_bucket|kubelet_image_pull_duration_seconds_sum|kubelet_image_pull_duration_seconds_count)$",
								},
							},
						},
						map[string]any{
							"job_name":          "cadvisor",
							"honor_labels":      true,
							"scrape_interval":   "30s",
							"scheme":            "https",
							"bearer_token_file": "/var/run/secrets/kubernetes.io/serviceaccount/token",
							"tls_config": map[string]any{
								// "ca_file": "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
								"insecure_skip_verify": true,
							},
							"metrics_path": "/metrics/cadvisor",
							"kubernetes_sd_configs": []any{
								map[string]any{"role": "node"},
							},
							"relabel_configs": []any{
								map[string]any{
									"action": "labelmap",
									"regex":  "__meta_kubernetes_node_label_(.+)",
								},
								map[string]any{
									"source_labels": []string{"__meta_kubernetes_node_name"},
									"action":        "replace",
									"target_label":  "instance",
								},
								map[string]any{
									"source_labels": []string{"__meta_kubernetes_node_name"},
									"action":        "replace",
									"target_label":  "node",
								},
								map[string]any{
									"target_label": "type",
									"replacement":  "shoot",
								},
							},
							"metric_relabel_configs": []any{
								// Extract systemd_service_name from id label
								map[string]any{
									"source_labels": []string{"id"},
									"action":        "replace",
									"regex":         "^/system\\.slice/(.+)\\.service$",
									"target_label":  "systemd_service_name",
								},
								// Extract container from id label (for systemd services)
								map[string]any{
									"source_labels": []string{"id"},
									"action":        "replace",
									"regex":         "^/system\\.slice/(.+)\\.service$",
									// "replacement": "$1",
									"target_label": "container",
								},
								// Keep only exact metric names from original
								map[string]any{
									"source_labels": []string{"__name__"},
									"action":        "keep",
									"regex":         "^(up|container_cpu_cfs_periods_total|container_cpu_cfs_throttled_seconds_total|container_cpu_cfs_throttled_periods_total|container_cpu_usage_seconds_total|container_fs_inodes_total|container_fs_limit_bytes|container_fs_usage_bytes|container_fs_reads_total|container_fs_writes_total|container_last_seen|container_memory_working_set_bytes|container_network_receive_bytes_total|container_network_transmit_bytes_total)$",
								},
								// Keep only kube-system namespace (and empty for systemd containers)
								map[string]any{
									"source_labels": []string{"namespace"},
									"action":        "keep",
									"regex":         "(^$|^kube-system$)",
								},
								// Drop POD container metrics (not network)
								map[string]any{
									"source_labels": []string{"container", "__name__"},
									"action":        "drop",
									"regex":         "POD;(container_cpu_cfs_periods_total|container_cpu_cfs_throttled_seconds_total|container_cpu_cfs_throttled_periods_total|container_cpu_usage_seconds_total|container_fs_inodes_total|container_fs_limit_bytes|container_fs_usage_bytes|container_last_seen|container_memory_working_set_bytes)",
								},
								// Network interface filtering - keep
								map[string]any{
									"source_labels": []string{"__name__", "container", "interface", "id"},
									"action":        "keep",
									"regex":         "container_network.+;;(eth0;/.+|(en.+|tunl0|eth0);/)|.+;.+;.*;.*",
								},
								// Network interface filtering - drop
								map[string]any{
									"source_labels": []string{"__name__", "container", "interface"},
									"action":        "drop",
									"regex":         "container_network.+;POD;(.{5,}|tun0|en.+)",
								},
								// Set host_network label for container_network metrics on host
								map[string]any{
									"source_labels": []string{"__name__", "id"},
									"regex":         "container_network.+;/",
									"replacement":   "true",
									"target_label":  "host_network",
								},
								// Drop id label
								map[string]any{
									"regex":  "^id$",
									"action": "labeldrop",
								},
							},
						},
						map[string]any{
							"job_name":        "node-exporter",
							"honor_labels":    true,
							"scrape_interval": "30s",
							// Note: node-exporter uses plain HTTP without authentication
							"kubernetes_sd_configs": []any{
								map[string]any{
									"role": "endpoints",
									"namespaces": map[string]any{
										"names": []string{"kube-system"},
									},
								},
							},
							"relabel_configs": []any{
								map[string]any{
									"source_labels": []string{"__meta_kubernetes_service_name"},
									"action":        "keep",
									"regex":         "node-exporter",
								},
								map[string]any{
									"source_labels": []string{"__meta_kubernetes_endpoint_port_name"},
									"action":        "keep",
									"regex":         "metrics",
								},
								map[string]any{
									"source_labels": []string{"__meta_kubernetes_pod_node_name"},
									"action":        "replace",
									"target_label":  "instance",
								},
								map[string]any{
									"source_labels": []string{"__meta_kubernetes_pod_node_name"},
									"action":        "replace",
									"target_label":  "node",
								},
							},
							"metric_relabel_configs": []any{
								map[string]any{
									"source_labels": []string{"__name__"},
									"action":        "keep",
									"regex":         "^(up|node_boot_time_seconds|node_cpu_seconds_total|node_disk_read_bytes_total|node_disk_written_bytes_total|node_disk_io_time_weighted_seconds_total|node_disk_io_time_seconds_total|node_disk_write_time_seconds_total|node_disk_writes_completed_total|node_disk_read_time_seconds_total|node_disk_reads_completed_total|node_filesystem_avail_bytes|node_filesystem_files|node_filesystem_files_free|node_filesystem_free_bytes|node_filesystem_readonly|node_filesystem_size_bytes|node_load1|node_load15|node_load5|node_memory_.+|node_nf_conntrack_.+|node_scrape_collector_duration_seconds|node_scrape_collector_success|process_max_fds|process_open_fds)$",
								},
							},
						},
					},
				},
			},
		},
		"processors": map[string]any{
			"batch": map[string]any{
				"timeout":         "10s",
				"send_batch_size": 1000,
			},
		},
		"exporters": map[string]any{
			"prometheus": map[string]any{
				"endpoint": "0.0.0.0:8080",
				// "namespace": "",
				"send_timestamps":   true,
				"metric_expiration": "5m",
			},
		},
		"service": map[string]any{
			"pipelines": map[string]any{
				"metrics": map[string]any{
					"receivers":  []string{"prometheus"},
					"processors": []string{"batch"},
					"exporters":  []string{"prometheus"},
				},
			},
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func getLabels() map[string]string {
	return map[string]string{
		labelKeyComponent: componentName,
	}
}
