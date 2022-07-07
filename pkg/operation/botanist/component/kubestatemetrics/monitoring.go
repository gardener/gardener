// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubestatemetrics

import (
	"bytes"
	"text/template"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"

	"github.com/Masterminds/sprig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

const (
	monitoringPrometheusJobName     = "kube-state-metrics"
	monitoringPrometheusJobNameSeed = "kube-state-metrics-seed"

	monitoringMetricKubeDaemonSetMetadataGeneration                                                 = "kube_daemonset_metadata_generation"
	monitoringMetricKubeDaemonSetStatusCurrentNumberScheduled                                       = "kube_daemonset_status_current_number_scheduled"
	monitoringMetricKubeDaemonSetStatusDesiredNumberScheduled                                       = "kube_daemonset_status_desired_number_scheduled"
	monitoringMetricKubeDaemonSetStatusNumberAvailable                                              = "kube_daemonset_status_number_available"
	monitoringMetricKubeDaemonSetStatusNumberUnavailable                                            = "kube_daemonset_status_number_unavailable"
	monitoringMetricKubeDaemonSetStatusUpdatedNumberScheduled                                       = "kube_daemonset_status_updated_number_scheduled"
	monitoringMetricKubeDeploymentMetadataGeneration                                                = "kube_deployment_metadata_generation"
	monitoringMetricKubeDeploymentSpecReplicas                                                      = "kube_deployment_spec_replicas"
	monitoringMetricKubeDeploymentStatusObservedGeneration                                          = "kube_deployment_status_observed_generation"
	monitoringMetricKubeDeploymentStatusReplicas                                                    = "kube_deployment_status_replicas"
	monitoringMetricKubeDeploymentStatusReplicasAvailable                                           = "kube_deployment_status_replicas_available"
	monitoringMetricKubeDeploymentStatusReplicasUnavailable                                         = "kube_deployment_status_replicas_unavailable"
	monitoringMetricKubeDeploymentStatusReplicasUpdated                                             = "kube_deployment_status_replicas_updated"
	monitoringMetricKubeHorizontalPodAutoscalerSpecMaxReplicas                                      = "kube_horizontalpodautoscaler_spec_max_replicas"
	monitoringMetricKubeHorizontalPodAutoscalerSpecMinReplicas                                      = "kube_horizontalpodautoscaler_spec_min_replicas"
	monitoringMetricKubeHorizontalPodAutoscalerStatusCurrentReplicas                                = "kube_horizontalpodautoscaler_status_current_replicas"
	monitoringMetricKubeHorizontalPodAutoscalerStatusDesiredReplicas                                = "kube_horizontalpodautoscaler_status_desired_replicas"
	monitoringMetricKubeHorizontalPodAutoscalerStatusCondition                                      = "kube_horizontalpodautoscaler_status_condition"
	monitoringMetricKubeNodeInfo                                                                    = "kube_node_info"
	monitoringMetricKubeNodeLabels                                                                  = "kube_node_labels"
	monitoringMetricKubeNodeSpecUnschedulable                                                       = "kube_node_spec_unschedulable"
	monitoringMetricKubeNodeStatusAllocatable                                                       = "kube_node_status_allocatable"
	monitoringMetricKubeNodeStatusCapacity                                                          = "kube_node_status_capacity"
	monitoringMetricKubeNodeStatusCondition                                                         = "kube_node_status_condition"
	monitoringMetricKubePersistentVolumeClaimResourceRequestsStorageBytes                           = "kube_persistentvolumeclaim_resource_requests_storage_bytes"
	monitoringMetricKubePodContainerInfo                                                            = "kube_pod_container_info"
	monitoringMetricKubePodContainerResourceLimits                                                  = "kube_pod_container_resource_limits"
	monitoringMetricKubePodContainerResourceRequests                                                = "kube_pod_container_resource_requests"
	monitoringMetricKubePodContainerStatusRestartsTotal                                             = "kube_pod_container_status_restarts_total"
	monitoringMetricKubePodInfo                                                                     = "kube_pod_info"
	monitoringMetricKubePodLabels                                                                   = "kube_pod_labels"
	monitoringMetricKubePodStatusPhase                                                              = "kube_pod_status_phase"
	monitoringMetricKubePodStatusReady                                                              = "kube_pod_status_ready"
	monitoringMetricKubeReplicaSetMetadataGeneration                                                = "kube_replicaset_metadata_generation"
	monitoringMetricKubeReplicaSetOwner                                                             = "kube_replicaset_owner"
	monitoringMetricKubeReplicaSetSpecReplicas                                                      = "kube_replicaset_spec_replicas"
	monitoringMetricKubeReplicaSetStatusObservedGeneration                                          = "kube_replicaset_status_observed_generation"
	monitoringMetricKubeReplicaSetStatusReplicas                                                    = "kube_replicaset_status_replicas"
	monitoringMetricKubeReplicaSetStatusReadyReplicas                                               = "kube_replicaset_status_ready_replicas"
	monitoringMetricKubeStatefulSetMetadataGeneration                                               = "kube_statefulset_metadata_generation"
	monitoringMetricKubeStatefulSetReplicas                                                         = "kube_statefulset_replicas"
	monitoringMetricKubeStatefulSetStatusObservedGeneration                                         = "kube_statefulset_status_observed_generation"
	monitoringMetricKubeStatefulSetStatusReplicas                                                   = "kube_statefulset_status_replicas"
	monitoringMetricKubeStatefulSetStatusReplicasCurrent                                            = "kube_statefulset_status_replicas_current"
	monitoringMetricKubeStatefulSetStatusReplicasReady                                              = "kube_statefulset_status_replicas_ready"
	monitoringMetricKubeStatefulSetStatusReplicasUpdated                                            = "kube_statefulset_status_replicas_updated"
	monitoringMetricKubeVerticalPodAutoscalerStatusRecommendationContainerRecommendationsTarget     = "kube_verticalpodautoscaler_status_recommendation_containerrecommendations_target"
	monitoringMetricKubeVerticalPodAutoscalerStatusRecommendationContainerRecommendationsUpperBound = "kube_verticalpodautoscaler_status_recommendation_containerrecommendations_upperbound"
	monitoringMetricKubeVerticalPodAutoscalerStatusRecommendationContainerRecommendationsLowerBound = "kube_verticalpodautoscaler_status_recommendation_containerrecommendations_lowerbound"
	monitoringMetricKubeVerticalPodAutoscalerSpecUpdatePolicyUpdateMode                             = "kube_verticalpodautoscaler_spec_updatepolicy_updatemode"
)

var (
	centralMonitoringAllowedMetrics = []string{
		monitoringMetricKubeDaemonSetMetadataGeneration,
		monitoringMetricKubeDaemonSetStatusCurrentNumberScheduled,
		monitoringMetricKubeDaemonSetStatusDesiredNumberScheduled,
		monitoringMetricKubeDaemonSetStatusNumberAvailable,
		monitoringMetricKubeDaemonSetStatusNumberUnavailable,
		monitoringMetricKubeDaemonSetStatusUpdatedNumberScheduled,
		monitoringMetricKubeDeploymentMetadataGeneration,
		monitoringMetricKubeDeploymentSpecReplicas,
		monitoringMetricKubeDeploymentStatusObservedGeneration,
		monitoringMetricKubeDeploymentStatusReplicas,
		monitoringMetricKubeDeploymentStatusReplicasAvailable,
		monitoringMetricKubeDeploymentStatusReplicasUnavailable,
		monitoringMetricKubeDeploymentStatusReplicasUpdated,
		monitoringMetricKubeHorizontalPodAutoscalerSpecMaxReplicas,
		monitoringMetricKubeHorizontalPodAutoscalerSpecMinReplicas,
		monitoringMetricKubeHorizontalPodAutoscalerStatusCurrentReplicas,
		monitoringMetricKubeHorizontalPodAutoscalerStatusDesiredReplicas,
		monitoringMetricKubeHorizontalPodAutoscalerStatusCondition,
		monitoringMetricKubeNodeInfo,
		monitoringMetricKubeNodeLabels,
		monitoringMetricKubeNodeSpecUnschedulable,
		monitoringMetricKubeNodeStatusAllocatable,
		monitoringMetricKubeNodeStatusCapacity,
		monitoringMetricKubeNodeStatusCondition,
		monitoringMetricKubePersistentVolumeClaimResourceRequestsStorageBytes,
		monitoringMetricKubePodContainerInfo,
		monitoringMetricKubePodContainerResourceLimits,
		monitoringMetricKubePodContainerResourceRequests,
		monitoringMetricKubePodContainerStatusRestartsTotal,
		monitoringMetricKubePodInfo,
		monitoringMetricKubePodLabels,
		monitoringMetricKubePodStatusPhase,
		monitoringMetricKubePodStatusReady,
		monitoringMetricKubeStatefulSetMetadataGeneration,
		monitoringMetricKubeStatefulSetReplicas,
		monitoringMetricKubeStatefulSetStatusObservedGeneration,
		monitoringMetricKubeStatefulSetStatusReplicas,
		monitoringMetricKubeStatefulSetStatusReplicasCurrent,
		monitoringMetricKubeStatefulSetStatusReplicasReady,
		monitoringMetricKubeStatefulSetStatusReplicasUpdated,
	}

	shootMonitoringAllowedMetrics = []string{
		monitoringMetricKubeDaemonSetMetadataGeneration,
		monitoringMetricKubeDaemonSetStatusCurrentNumberScheduled,
		monitoringMetricKubeDaemonSetStatusDesiredNumberScheduled,
		monitoringMetricKubeDaemonSetStatusNumberAvailable,
		monitoringMetricKubeDaemonSetStatusNumberUnavailable,
		monitoringMetricKubeDaemonSetStatusUpdatedNumberScheduled,
		monitoringMetricKubeDeploymentMetadataGeneration,
		monitoringMetricKubeDeploymentSpecReplicas,
		monitoringMetricKubeDeploymentStatusObservedGeneration,
		monitoringMetricKubeDeploymentStatusReplicas,
		monitoringMetricKubeDeploymentStatusReplicasAvailable,
		monitoringMetricKubeDeploymentStatusReplicasUnavailable,
		monitoringMetricKubeDeploymentStatusReplicasUpdated,
		monitoringMetricKubeNodeInfo,
		monitoringMetricKubeNodeLabels,
		monitoringMetricKubeNodeSpecUnschedulable,
		monitoringMetricKubeNodeStatusAllocatable,
		monitoringMetricKubeNodeStatusCapacity,
		monitoringMetricKubeNodeStatusCondition,
		monitoringMetricKubePodContainerInfo,
		monitoringMetricKubePodContainerResourceLimits,
		monitoringMetricKubePodContainerResourceRequests,
		monitoringMetricKubePodContainerStatusRestartsTotal,
		monitoringMetricKubePodInfo,
		monitoringMetricKubePodLabels,
		monitoringMetricKubePodStatusPhase,
		monitoringMetricKubePodStatusReady,
		monitoringMetricKubeReplicaSetMetadataGeneration,
		monitoringMetricKubeReplicaSetOwner,
		monitoringMetricKubeReplicaSetSpecReplicas,
		monitoringMetricKubeReplicaSetStatusObservedGeneration,
		monitoringMetricKubeReplicaSetStatusReplicas,
		monitoringMetricKubeReplicaSetStatusReadyReplicas,
		monitoringMetricKubeStatefulSetMetadataGeneration,
		monitoringMetricKubeStatefulSetReplicas,
		monitoringMetricKubeStatefulSetStatusObservedGeneration,
		monitoringMetricKubeStatefulSetStatusReplicas,
		monitoringMetricKubeStatefulSetStatusReplicasCurrent,
		monitoringMetricKubeStatefulSetStatusReplicasReady,
		monitoringMetricKubeStatefulSetStatusReplicasUpdated,
		monitoringMetricKubeVerticalPodAutoscalerStatusRecommendationContainerRecommendationsTarget,
		monitoringMetricKubeVerticalPodAutoscalerStatusRecommendationContainerRecommendationsUpperBound,
		monitoringMetricKubeVerticalPodAutoscalerStatusRecommendationContainerRecommendationsLowerBound,
		monitoringMetricKubeVerticalPodAutoscalerSpecUpdatePolicyUpdateMode,
	}

	monitoringScrapeConfigTmpl = `job_name: {{ .jobName }}
honor_labels: false
# Service is used, because we only care about metric from one kube-state-metrics instance
# and not multiple in HA setup
kubernetes_sd_configs:
- role: service
  namespaces:
    names: [ {{ .serviceNamespace }} ]
relabel_configs:
- source_labels: [ __meta_kubernetes_service_label_` + labelKeyComponent + ` ]
  action: keep
  regex: ` + labelValueComponent + `
- source_labels: [ __meta_kubernetes_service_port_name ]
  action: keep
- source_labels: [ __meta_kubernetes_service_label_` + labelKeyType + ` ]
  regex: (.+)
  target_label: ` + labelKeyType + `
  replacement: ${1}
- target_label: instance
  replacement: kube-state-metrics
metric_relabel_configs:
{{- if .relabeledNamespace }}
- source_labels: [namespace]
  regex: {{ .relabeledNamespace }}
  action: keep
{{- end }}
{{- if .forShoot }}
# we make the shoot's pods in the shoot's namespace to appear as if they are in the kube-system namespace
- target_label: namespace
  replacement: ` + metav1.NamespaceSystem + `
{{- end }}
- source_labels: [ pod ]
  regex: ^.+\.tf-pod.+$
  action: drop
- source_labels: [ __name__ ]
  action: keep
  regex: ^({{ join "|" .allowedMetrics }})$
`
	monitoringScrapeConfigTemplate *template.Template

	monitoringAlertingRules = `groups:
- name: kube-state-metrics.rules
  rules:
  - alert: KubeStateMetricsShootDown
    expr: absent(up{job="` + monitoringPrometheusJobName + `", type="shoot"} == 1)
    for: 15m
    labels:
      service: kube-state-metrics-shoot
      severity: info
      visibility: operator
      type: seed
    annotations:
      summary: Kube-state-metrics for shoot cluster metrics is down.
      description: There are no running kube-state-metric pods for the shoot cluster. No kubernetes resource metrics can be scraped.

  - alert: KubeStateMetricsSeedDown
    expr: absent(up{job="` + monitoringPrometheusJobNameSeed + `", type="seed"} == 1)
    for: 15m
    labels:
      service: kube-state-metrics-seed
      severity: critical
      visibility: operator
      type: seed
    annotations:
      summary: Kube-state-metrics for seed cluster metrics is down.
      description: There are no running kube-state-metric pods for the seed cluster. No kubernetes resource metrics can be scraped.

  - alert: NoWorkerNodes
    expr: sum(` + monitoringMetricKubeNodeSpecUnschedulable + `) == count(` + monitoringMetricKubeNodeInfo + `) or absent(` + monitoringMetricKubeNodeInfo + `)
    for: 25m # MCM timeout + grace period to allow self healing before firing alert
    labels:
      service: nodes
      severity: blocker
      visibility: all
    annotations:
      description: There are no worker nodes in the cluster or all of the worker nodes in the cluster are not schedulable.
      summary: No nodes available. Possibly all workloads down.

  - record: shoot:kube_node_status_capacity_cpu_cores:sum
    expr: sum(` + monitoringMetricKubeNodeStatusCapacity + `{resource="cpu",unit="core"})

  - record: shoot:kube_node_status_capacity_memory_bytes:sum
    expr: sum(` + monitoringMetricKubeNodeStatusCapacity + `{resource="memory",unit="byte"})

  - record: shoot:machine_types:sum
    expr: sum(` + monitoringMetricKubeNodeLabels + `) by (label_beta_kubernetes_io_instance_type)

  - record: shoot:node_operating_system:sum
    expr: sum(` + monitoringMetricKubeNodeInfo + `) by (os_image, kernel_version)

  # Mitigation for extension dashboards.
  # TODO(istvanballok): Remove in a future version. For more details, see https://github.com/gardener/gardener/pull/6224.
  - record: kube_pod_container_resource_limits_cpu_cores
    expr: ` + monitoringMetricKubePodContainerResourceLimits + `{resource="cpu", unit="core"}

  - record: kube_pod_container_resource_requests_cpu_cores
    expr: ` + monitoringMetricKubePodContainerResourceRequests + `{resource="cpu", unit="core"}

  - record: kube_pod_container_resource_limits_memory_bytes
    expr: ` + monitoringMetricKubePodContainerResourceLimits + `{resource="memory", unit="byte"}

  - record: kube_pod_container_resource_requests_memory_bytes
    expr: ` + monitoringMetricKubePodContainerResourceRequests + `{resource="memory", unit="byte"}
`
)

func init() {
	var err error

	monitoringScrapeConfigTemplate, err = template.
		New("monitoring-scrape-config").
		Funcs(sprig.TxtFuncMap()).
		Parse(monitoringScrapeConfigTmpl)
	utilruntime.Must(err)
}

// CentralMonitoringConfiguration returns scrape configs for the central Prometheus.
func CentralMonitoringConfiguration() (component.CentralMonitoringConfig, error) {
	var scrapeConfig bytes.Buffer

	if err := monitoringScrapeConfigTemplate.Execute(&scrapeConfig, map[string]interface{}{
		"jobName":          monitoringPrometheusJobName,
		"serviceNamespace": v1beta1constants.GardenNamespace,
		"forShoot":         false,
		"allowedMetrics":   centralMonitoringAllowedMetrics,
	}); err != nil {
		return component.CentralMonitoringConfig{}, err
	}

	return component.CentralMonitoringConfig{ScrapeConfigs: []string{scrapeConfig.String()}}, nil
}

// ScrapeConfigs returns the scrape configurations for Prometheus.
func (k *kubeStateMetrics) ScrapeConfigs() ([]string, error) {
	var (
		scrapeConfig     bytes.Buffer
		scrapeConfigSeed bytes.Buffer
	)

	if err := monitoringScrapeConfigTemplate.Execute(&scrapeConfig, map[string]interface{}{
		"jobName":          monitoringPrometheusJobName,
		"serviceNamespace": k.namespace,
		"forShoot":         true,
		"allowedMetrics":   shootMonitoringAllowedMetrics,
	}); err != nil {
		return nil, err
	}
	if err := monitoringScrapeConfigTemplate.Execute(&scrapeConfigSeed, map[string]interface{}{
		"jobName":            monitoringPrometheusJobNameSeed,
		"serviceNamespace":   v1beta1constants.GardenNamespace,
		"relabeledNamespace": k.namespace,
		"forShoot":           true,
		"allowedMetrics":     shootMonitoringAllowedMetrics,
	}); err != nil {
		return nil, err
	}

	return []string{
		scrapeConfig.String(),
		scrapeConfigSeed.String(),
	}, nil
}

// AlertingRules returns the alerting rules for AlertManager.
func (k *kubeStateMetrics) AlertingRules() (map[string]string, error) {
	return map[string]string{"kube-state-metrics.rules.yaml": monitoringAlertingRules}, nil
}
