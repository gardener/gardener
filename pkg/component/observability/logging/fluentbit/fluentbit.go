// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fluentbit

import (
	"context"
	"fmt"
	"time"

	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentcustomresources"
	valiconstants "github.com/gardener/gardener/pkg/component/observability/logging/vali/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/aggregate"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	managedResourceName = "fluent-bit"
	fluentBitConfigName = "fluent-bit-config"
)

// Values is the values for fluent-bit configurations
type Values struct {
	// Image is the fluent-bit image.
	Image string
	// InitContainerImage is the fluent-bit init container image.
	InitContainerImage string
	// VailEnabled specifies whether vali is used and should be configured as a ClusterOutput.
	ValiEnabled bool
	// PriorityClassName is the name of the priority class of the fluent-bit.
	PriorityClassName string
}

type fluentBit struct {
	client    client.Client
	namespace string
	values    Values
}

// New creates a new instance of fluent-bit deployer.
func New(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &fluentBit{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

func (f *fluentBit) Deploy(ctx context.Context) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.DaemonSetNameFluentBit + "-lua-config",
				Namespace: f.namespace,
			},
			// spellchecker:off
			Data: map[string]string{
				"modify_severity.lua": `
function cb_modify(tag, timestamp, record)
  local unified_severity = cb_modify_unify_severity(record)

  if not unified_severity then
    return 0, 0, 0
  end

  return 1, timestamp, record
end

function cb_modify_unify_severity(record)
  local modified = false
  local severity = record["severity"]
  if severity == nil or severity == "" then
	return modified
  end

  severity = trim(severity):upper()

  if severity == "I" or severity == "INF" or severity == "INFO" then
    record["severity"] = "INFO"
    modified = true
  elseif severity == "W" or severity == "WRN" or severity == "WARN" or severity == "WARNING" then
    record["severity"] = "WARN"
    modified = true
  elseif severity == "E" or severity == "ERR" or severity == "ERROR" or severity == "EROR" then
    record["severity"] = "ERR"
    modified = true
  elseif severity == "D" or severity == "DBG" or severity == "DEBUG" then
    record["severity"] = "DBG"
    modified = true
  elseif severity == "N" or severity == "NOTICE" then
    record["severity"] = "NOTICE"
    modified = true
  elseif severity == "F" or severity == "FATAL" then
    record["severity"] = "FATAL"
    modified = true
  end

  return modified
end

function trim(s)
  return (s:gsub("^%s*(.-)%s*$", "%1"))
end`,
				"add_tag_to_record.lua": `
function add_tag_to_record(tag, timestamp, record)
  record["tag"] = tag
  return 1, timestamp, record
end
`,
			},
			// spellchecker:on
		}
		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: monitoringutils.ConfigObjectMeta("fluent-bit", f.namespace, aggregate.Label),
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: getLabels()},
				Endpoints: []monitoringv1.Endpoint{{
					Port: "metrics",
					RelabelConfigs: []monitoringv1.RelabelConfig{
						{
							TargetLabel: "__metrics_path__",
							Replacement: ptr.To("/api/v1/metrics/prometheus"),
						},
						{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_pod_label_(.+)`,
						},
					},
					MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
						"fluentbit_input_bytes_total",
						"fluentbit_input_records_total",
						"fluentbit_output_proc_bytes_total",
						"fluentbit_output_proc_records_total",
						"fluentbit_output_errors_total",
						"fluentbit_output_retries_total",
						"fluentbit_output_retries_failed_total",
						"fluentbit_filter_add_records_total",
						"fluentbit_filter_drop_records_total",
					),
				}},
			},
		}
		serviceMonitorPlugin = &monitoringv1.ServiceMonitor{
			ObjectMeta: monitoringutils.ConfigObjectMeta("fluent-bit-output-plugin", f.namespace, aggregate.Label),
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: getLabels()},
				Endpoints: []monitoringv1.Endpoint{{
					Port: "metrics-plugin",
					RelabelConfigs: []monitoringv1.RelabelConfig{
						// This service monitor is targeting the fluent-bit service. Without explicitly overriding the
						// job label, prometheus-operator would choose job=fluent-bit (service name).
						{
							Action:      "replace",
							Replacement: ptr.To("fluent-bit-output-plugin"),
							TargetLabel: "job",
						},
						{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_pod_label_(.+)`,
						},
					},
					MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
						"valitail_dropped_entries_total",
						"fluentbit_vali_gardener_errors_total",
						"fluentbit_vali_gardener_logs_without_metadata_total",
						"fluentbit_vali_gardener_incoming_logs_total",
						"fluentbit_vali_gardener_incoming_logs_with_endpoint_total",
						"fluentbit_vali_gardener_forwarded_logs_total",
						"fluentbit_vali_gardener_dropped_logs_total",
					),
				}},
			},
		}
		prometheusRule = &monitoringv1.PrometheusRule{
			ObjectMeta: monitoringutils.ConfigObjectMeta("fluent-bit", f.namespace, aggregate.Label),
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{
					Name: "fluent-bit.rules",
					Rules: []monitoringv1.Rule{
						{
							Alert: "FluentBitDown",
							Expr:  intstr.FromString(`absent(up{job="fluent-bit"} == 1)`),
							For:   ptr.To(monitoringv1.Duration("15m")),
							Labels: map[string]string{
								"service":    "logging",
								"severity":   "warning",
								"type":       "seed",
								"visibility": "operator",
							},
							Annotations: map[string]string{
								"description": "There are no fluent-bit pods running on seed: {{$externalLabels.seed}}. No logs will be collected.",
								"summary":     "Fluent-bit is down",
							},
						},
						{
							Alert: "FluentBitIdleInputPlugins",
							Expr:  intstr.FromString(`sum by (pod) (increase(fluentbit_input_bytes_total{pod=~"fluent-bit.*"}[4m])) == 0`),
							For:   ptr.To(monitoringv1.Duration("6h")),
							Labels: map[string]string{
								"service":    "logging",
								"severity":   "warning",
								"type":       "seed",
								"visibility": "operator",
							},
							Annotations: map[string]string{
								"description": "The input plugins of Fluent-bit pod {{$labels.pod}} running on seed {{$externalLabels.seed}} haven't collected any logs for the last 6 hours.",
								"summary":     "Fluent-bit input plugins haven't process any data for the past 6 hours",
							},
						},
						{
							Alert: "FluentBitReceivesLogsWithoutMetadata",
							Expr:  intstr.FromString(`sum by (pod) (increase(fluentbit_vali_gardener_logs_without_metadata_total[4m])) > 0`),
							Labels: map[string]string{
								"service":    "logging",
								"severity":   "warning",
								"type":       "seed",
								"visibility": "operator",
							},
							Annotations: map[string]string{
								"description": "{{$labels.pod}} receives logs without metadata on seed: {{$externalLabels.seed}}. These logs will be dropped.",
								"summary":     "Fluent-bit receives logs without metadata",
							},
						},
						{
							Alert: "FluentBitSendsOoOLogs",
							Expr:  intstr.FromString(`sum by (pod) (increase(prometheus_target_scrapes_sample_out_of_order_total[4m])) > 0`),
							Labels: map[string]string{
								"service":    "logging",
								"severity":   "warning",
								"type":       "seed",
								"visibility": "operator",
							},
							Annotations: map[string]string{
								"description": "{{$labels.pod}} on seed: {{$externalLabels.seed}} sends OutOfOrder logs to the Vali. These logs will be dropped.",
								"summary":     "Fluent-bit sends OoO logs",
							},
						},
						{
							Alert: "FluentBitGardenerValiPluginErrors",
							Expr:  intstr.FromString(`sum by (pod) (increase(fluentbit_vali_gardener_errors_total[4m])) > 0`),
							Labels: map[string]string{
								"service":    "logging",
								"severity":   "warning",
								"type":       "seed",
								"visibility": "operator",
							},
							Annotations: map[string]string{
								"description": "There are errors in the {{$labels.pod}} GardenerVali plugin on seed: {{$externalLabels.seed}}.",
								"summary":     "Errors in Fluent-bit GardenerVali plugin",
							},
						},
					},
				}},
			},
		}
	)

	resources := []client.Object{
		configMap,
		f.getFluentBit(),
		f.getClusterFluentBitConfig(),
		serviceMonitor,
		serviceMonitorPlugin,
		prometheusRule,
	}

	if f.values.ValiEnabled {
		resources = append(resources, fluentcustomresources.GetDefaultClusterOutput(getCustomResourcesLabels()))
	}

	for _, clusterInput := range fluentcustomresources.GetClusterInputs(getCustomResourcesLabels()) {
		resources = append(resources, clusterInput)
	}

	for _, clusterFilter := range fluentcustomresources.GetClusterFilters(configMap.Name, getCustomResourcesLabels()) {
		resources = append(resources, clusterFilter)
	}

	for _, clusterParser := range fluentcustomresources.GetClusterParsers(getCustomResourcesLabels()) {
		resources = append(resources, clusterParser)
	}

	serializedResources, err := registry.AddAllAndSerialize(resources...)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, f.client, f.namespace, managedResourceName, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, serializedResources)
}

func (f *fluentBit) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, f.client, f.namespace, managedResourceName)
}

var timeoutWaitForManagedResources = 2 * time.Minute

func (f *fluentBit) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, f.client, f.namespace, managedResourceName)
}

func (f *fluentBit) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, f.client, f.namespace, managedResourceName)
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:                             v1beta1constants.DaemonSetNameFluentBit,
		v1beta1constants.LabelRole:                            v1beta1constants.LabelLogging,
		v1beta1constants.GardenRole:                           v1beta1constants.GardenRoleLogging,
		v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		gardenerutils.NetworkPolicyLabel(valiconstants.ServiceName, valiconstants.ValiPort): v1beta1constants.LabelNetworkPolicyAllowed,
		"networking.resources.gardener.cloud/to-all-shoots-logging-tcp-3100":                v1beta1constants.LabelNetworkPolicyAllowed,
	}
}

func getCustomResourcesLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelKeyCustomLoggingResource: v1beta1constants.LabelValueCustomLoggingResource,
	}
}

func (f *fluentBit) getFluentBit() *fluentbitv1alpha2.FluentBit {
	annotations := map[string]string{
		resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationPrefix + v1beta1constants.LabelNetworkPolicySeedScrapeTargets + resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationSuffix: `[{"port":2020,"protocol":"TCP"},{"port":2021,"protocol":"TCP"}]`,
	}

	return &fluentbitv1alpha2.FluentBit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v-%v", v1beta1constants.DaemonSetNameFluentBit, utils.ComputeSHA256Hex([]byte(fmt.Sprintf("%v%v", getLabels(), annotations)))[:5]),
			Namespace: f.namespace,
			Labels:    getLabels(),
		},
		Spec: fluentbitv1alpha2.FluentBitSpec{
			FluentBitConfigName: fluentBitConfigName,
			Image:               f.values.Image,
			Command: []string{
				"/fluent-bit/bin/fluent-bit-watcher",
				"-e",
				"/fluent-bit/plugins/out_vali.so",
				"-c",
				"/fluent-bit/config/fluent-bit.conf",
			},
			ContainerSecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: ptr.To(false),
			},
			PriorityClassName: f.values.PriorityClassName,
			Ports: []corev1.ContainerPort{
				{
					Name:          "metrics-plugin",
					ContainerPort: 2021,
					Protocol:      "TCP",
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("650Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("150m"),
					corev1.ResourceMemory: resource.MustParse("200Mi"),
				},
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/api/v1/metrics/prometheus",
						Port: intstr.FromInt32(2020),
					},
				},
				PeriodSeconds: 10,
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/healthz",
						Port: intstr.FromInt32(2021),
					},
				},
				PeriodSeconds:       300,
				InitialDelaySeconds: 90,
			},
			Tolerations: []corev1.Toleration{
				{
					Key:    "node-role.kubernetes.io/master",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "node-role.kubernetes.io/control-plane",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "runlogjournal",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/run/log/journal",
						},
					},
				},
				{
					Name: "varfluent",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/fluentbit",
						},
					},
				},
				{
					Name: "plugins",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			VolumesMounts: []corev1.VolumeMount{
				{
					Name:      "runlogjournal",
					MountPath: "/run/log/journal",
				},
				{
					Name:      "varfluent",
					MountPath: "/var/fluentbit",
				},
				{
					Name:      "plugins",
					MountPath: "/fluent-bit/plugins",
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:  "install-plugin",
					Image: f.values.InitContainerImage,
					Command: []string{
						"cp",
						"/source/plugins/.",
						"/plugins",
					},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "plugins",
							MountPath: "/plugins",
						},
					},
				},
			},
			RBACRules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"extensions.gardener.cloud"},
					Resources: []string{"clusters"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
			Service: fluentbitv1alpha2.FluentBitService{
				Name:        v1beta1constants.DaemonSetNameFluentBit,
				Annotations: annotations,
				Labels:      getLabels(),
			},
		},
	}
}

func (f *fluentBit) getClusterFluentBitConfig() *fluentbitv1alpha2.ClusterFluentBitConfig {
	return &fluentbitv1alpha2.ClusterFluentBitConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: fluentBitConfigName,
			Labels: map[string]string{
				"app.kubernetes.io/name": v1beta1constants.DaemonSetNameFluentBit,
			},
		},
		Spec: fluentbitv1alpha2.FluentBitConfigSpec{
			Service: &fluentbitv1alpha2.Service{
				FlushSeconds: ptr.To[float64](30),
				Daemon:       ptr.To(false),
				LogLevel:     "error",
				ParsersFile:  "parsers.conf",
				HttpServer:   ptr.To(true),
				HttpListen:   "0.0.0.0",
				HttpPort:     ptr.To[int32](2020),
			},
			InputSelector: metav1.LabelSelector{
				MatchLabels: getCustomResourcesLabels(),
			},
			FilterSelector: metav1.LabelSelector{
				MatchLabels: getCustomResourcesLabels(),
			},
			ParserSelector: metav1.LabelSelector{
				MatchLabels: getCustomResourcesLabels(),
			},
			OutputSelector: metav1.LabelSelector{
				MatchLabels: getCustomResourcesLabels(),
			},
		},
	}
}
