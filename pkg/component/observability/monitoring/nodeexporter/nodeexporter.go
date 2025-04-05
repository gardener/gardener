// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeexporter

import (
	"context"
	"fmt"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	managedResourceName = "shoot-core-node-exporter"
	name                = "node-exporter"

	labelKeyComponent = "component"
	labelValue        = "node-exporter"

	portNameMetrics = "metrics"
	portMetrics     = int32(16909)

	volumeNameHost                   = "host"
	volumeMountPathHost              = "/host"
	volumeNameTextFileCollector      = "textfile"
	volumeMountPathTextFileCollector = "/textfile-collector"

	hostPathTextFileCollector = "/var/lib/node-exporter/textfile-collector"
)

// Values is a set of configuration values for the node-exporter component.
type Values struct {
	// Image is the container image used for node-exporter.
	Image string
	// VPAEnabled marks whether VerticalPodAutoscaler is enabled for the shoot.
	VPAEnabled bool
}

// New creates a new instance of DeployWaiter for node-exporter.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &nodeExporter{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type nodeExporter struct {
	client    client.Client
	namespace string
	values    Values
}

func (n *nodeExporter) Deploy(ctx context.Context) error {
	scrapeConfig := n.emptyScrapeConfig()
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, n.client, scrapeConfig, func() error {
		metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", shoot.Label)
		scrapeConfig.Spec = shoot.ClusterComponentScrapeConfigSpec(
			name,
			shoot.KubernetesServiceDiscoveryConfig{
				Role:             monitoringv1alpha1.KubernetesRoleEndpoint,
				ServiceName:      name,
				EndpointPortName: portNameMetrics,
			},
			"node_boot_time_seconds",
			"node_cpu_seconds_total",
			"node_disk_read_bytes_total",
			"node_disk_written_bytes_total",
			"node_disk_io_time_weighted_seconds_total",
			"node_disk_io_time_seconds_total",
			"node_disk_write_time_seconds_total",
			"node_disk_writes_completed_total",
			"node_disk_read_time_seconds_total",
			"node_disk_reads_completed_total",
			"node_filesystem_avail_bytes",
			"node_filesystem_files",
			"node_filesystem_files_free",
			"node_filesystem_free_bytes",
			"node_filesystem_readonly",
			"node_filesystem_size_bytes",
			"node_load1",
			"node_load15",
			"node_load5",
			"node_memory_.+",
			"node_nf_conntrack_.+",
			"node_scrape_collector_duration_seconds",
			"node_scrape_collector_success",
			"process_max_fds",
			"process_open_fds",
		)
		return nil
	}); err != nil {
		return err
	}

	prometheusRule := n.emptyPrometheusRule()
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, n.client, prometheusRule, func() error {
		metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", shoot.Label)
		prometheusRule.Spec = monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name: "node-exporter.rules",
				Rules: []monitoringv1.Rule{
					{
						Alert: "NodeExporterDown",
						Expr:  intstr.FromString(`absent(up{job="` + name + `"} == 1)`),
						For:   ptr.To(monitoringv1.Duration("1h")),
						Labels: map[string]string{
							"service":    name,
							"severity":   "warning",
							"type":       "shoot",
							"visibility": "owner",
						},
						Annotations: map[string]string{
							"summary":     "NodeExporter down or unreachable",
							"description": "The NodeExporter has been down or unreachable from Prometheus for more than 1 hour.",
						},
					},
					{
						Alert: "K8SNodeOutOfDisk",
						Expr:  intstr.FromString(`kube_node_status_condition{condition="OutOfDisk", status="true"} == 1`),
						For:   ptr.To(monitoringv1.Duration("1h")),
						Labels: map[string]string{
							"service":    name,
							"severity":   "critical",
							"type":       "shoot",
							"visibility": "owner",
						},
						Annotations: map[string]string{
							"summary":     "Node ran out of disk space.",
							"description": "Node {{$labels.node}} has run out of disk space.",
						},
					},
					{
						Alert: "K8SNodeMemoryPressure",
						Expr:  intstr.FromString(`kube_node_status_condition{condition="MemoryPressure", status="true"} == 1`),
						For:   ptr.To(monitoringv1.Duration("1h")),
						Labels: map[string]string{
							"service":    name,
							"severity":   "warning",
							"type":       "shoot",
							"visibility": "owner",
						},
						Annotations: map[string]string{
							"summary":     "Node is under memory pressure.",
							"description": "Node {{$labels.node}} is under memory pressure.",
						},
					},
					{
						Alert: "K8SNodeDiskPressure",
						Expr:  intstr.FromString(`kube_node_status_condition{condition="DiskPressure", status="true"} == 1`),
						For:   ptr.To(monitoringv1.Duration("1h")),
						Labels: map[string]string{
							"service":    name,
							"severity":   "warning",
							"type":       "shoot",
							"visibility": "owner",
						},
						Annotations: map[string]string{
							"summary":     "Node is under disk pressure.",
							"description": "Node {{$labels.node}} is under disk pressure.",
						},
					},
					{
						Record: "instance:conntrack_entries_usage:percent",
						Expr:   intstr.FromString(`(node_nf_conntrack_entries / node_nf_conntrack_entries_limit) * 100`),
					},
					{
						Alert: "VMRootfsFull",
						Expr:  intstr.FromString(`node_filesystem_free{mountpoint="/"} < 1024`),
						For:   ptr.To(monitoringv1.Duration("1h")),
						Labels: map[string]string{
							"service":    name,
							"severity":   "critical",
							"type":       "shoot",
							"visibility": "owner",
						},
						Annotations: map[string]string{
							"description": "Root filesystem device on instance {{$labels.instance}} is almost full.",
							"summary":     "Node's root filesystem is almost full",
						},
					},
					{
						Alert: "VMConntrackTableFull",
						Expr:  intstr.FromString(`instance:conntrack_entries_usage:percent > 90`),
						For:   ptr.To(monitoringv1.Duration("1h")),
						Labels: map[string]string{
							"service":    name,
							"severity":   "critical",
							"type":       "shoot",
							"visibility": "owner",
						},
						Annotations: map[string]string{
							"description": "The nf_conntrack table is {{$value}}% full.",
							"summary":     "Number of tracked connections is near the limit",
						},
					},
					{
						Record: "shoot:kube_node_info:count",
						Expr:   intstr.FromString(`count(kube_node_info{type="shoot"})`),
					},
					// This recording rule creates a series for nodes with less than 5% free inodes on a not read only mount point.
					// The series exists only if there are less than 5% free inodes,
					// to keep the cardinality of these federated metrics manageable.
					// Otherwise, we would get a series for each node in each shoot in the federating Prometheus.
					{
						Record: "shoot:node_filesystem_files_free:percent",
						Expr:   intstr.FromString(`sum by (node, mountpoint) (node_filesystem_files_free / node_filesystem_files * 100 < 5 and node_filesystem_readonly == 0)`),
					},
				},
			}},
		}
		return nil
	}); err != nil {
		return err
	}

	data, err := n.computeResourcesData()
	if err != nil {
		return err
	}
	return managedresources.CreateForShoot(ctx, n.client, n.namespace, managedResourceName, managedresources.LabelValueGardener, false, data)
}

func (n *nodeExporter) Destroy(ctx context.Context) error {
	if err := kubernetesutils.DeleteObjects(ctx, n.client,
		n.emptyScrapeConfig(),
		n.emptyPrometheusRule(),
	); err != nil {
		return err
	}

	return managedresources.DeleteForShoot(ctx, n.client, n.namespace, managedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (n *nodeExporter) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, n.client, n.namespace, managedResourceName)
}

func (n *nodeExporter) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, n.client, n.namespace, managedResourceName)
}

func (n *nodeExporter) emptyScrapeConfig() *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{ObjectMeta: monitoringutils.ConfigObjectMeta(name, n.namespace, shoot.Label)}
}

func (n *nodeExporter) emptyPrometheusRule() *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{ObjectMeta: monitoringutils.ConfigObjectMeta(name, n.namespace, shoot.Label)}
}

func (n *nodeExporter) computeResourcesData() (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-exporter",
				Namespace: metav1.NamespaceSystem,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceSystem,
				Labels:    getLabels(),
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: corev1.ClusterIPNone,
				Ports: []corev1.ServicePort{
					{
						Name:     portNameMetrics,
						Port:     portMetrics,
						Protocol: corev1.ProtocolTCP,
					},
				},
				Selector: getLabels(),
			},
		}

		daemonSet = &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: metav1.NamespaceSystem,
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.GardenRole:     v1beta1constants.GardenRoleMonitoring,
					managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
				}),
			},
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
				RevisionHistoryLimit: ptr.To[int32](2),
				UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
					Type: appsv1.RollingUpdateDaemonSetStrategyType,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: utils.MergeStringMaps(getLabels(), map[string]string{
							v1beta1constants.GardenRole:                         v1beta1constants.GardenRoleMonitoring,
							managedresources.LabelKeyOrigin:                     managedresources.LabelValueGardener,
							v1beta1constants.LabelNetworkPolicyToPublicNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyShootFromSeed:    v1beta1constants.LabelNetworkPolicyAllowed,
						}),
					},
					Spec: corev1.PodSpec{
						Tolerations: []corev1.Toleration{
							{
								Effect:   corev1.TaintEffectNoSchedule,
								Operator: corev1.TolerationOpExists,
							},
							{
								Effect:   corev1.TaintEffectNoExecute,
								Operator: corev1.TolerationOpExists,
							},
						},
						HostNetwork:                  true,
						HostPID:                      true,
						PriorityClassName:            "system-cluster-critical",
						ServiceAccountName:           serviceAccount.Name,
						AutomountServiceAccountToken: ptr.To(false),
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: ptr.To(true),
							RunAsUser:    ptr.To[int64](65534),
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Containers: []corev1.Container{
							{
								Name:            name,
								Image:           n.values.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/bin/node_exporter",
									fmt.Sprintf("--web.listen-address=:%d", portMetrics),
									"--path.procfs=/host/proc",
									"--path.sysfs=/host/sys",
									"--path.rootfs=/host",
									"--path.udev.data=/host/run/udev/data",
									"--log.level=error",
									"--collector.disable-defaults",
									"--collector.conntrack",
									"--collector.cpu",
									"--collector.diskstats",
									"--collector.filefd",
									"--collector.filesystem",
									"--collector.filesystem.mount-points-exclude=^/(run|var)/.+$|^/(boot|dev|sys|usr)($|/.+$)",
									"--collector.loadavg",
									"--collector.meminfo",
									"--collector.uname",
									"--collector.stat",
									"--collector.pressure",
									"--collector.textfile",
									"--collector.textfile.directory=" + volumeMountPathTextFileCollector,
								},
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: portMetrics,
										Protocol:      corev1.ProtocolTCP,
										HostPort:      portMetrics,
										Name:          "scrape",
									},
								},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/",
											Port: intstr.FromInt32(portMetrics),
										},
									},
									InitialDelaySeconds: 5,
									TimeoutSeconds:      5,
								},
								ReadinessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/",
											Port: intstr.FromInt32(portMetrics),
										},
									},
									InitialDelaySeconds: 5,
									TimeoutSeconds:      5,
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("50m"),
										corev1.ResourceMemory: resource.MustParse("50Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      volumeNameHost,
										ReadOnly:  true,
										MountPath: volumeMountPathHost,
									},
									{
										Name:      volumeNameTextFileCollector,
										ReadOnly:  true,
										MountPath: volumeMountPathTextFileCollector,
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: volumeNameHost,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/",
									},
								},
							},
							{
								Name: volumeNameTextFileCollector,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: hostPathTextFileCollector,
									},
								},
							},
						},
					},
				},
			},
		}

		vpa *vpaautoscalingv1.VerticalPodAutoscaler
	)

	if n.values.VPAEnabled {
		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
		vpaControlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-exporter",
				Namespace: metav1.NamespaceSystem,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("50Mi"),
							},
							ControlledValues: &vpaControlledValues,
						},
					},
				},
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "DaemonSet",
					Name:       daemonSet.Name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
			},
		}
	}

	return registry.AddAllAndSerialize(
		serviceAccount,
		service,
		daemonSet,
		vpa,
	)
}

func getLabels() map[string]string {
	return map[string]string{
		labelKeyComponent: labelValue,
	}
}
