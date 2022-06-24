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
	"strings"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
)

const (
	monitoringPrometheusJobName = "kube-state-metrics"

	monitoringMetricKubePersistentVolumeClaimResourceRequestsStorageBytes = "kube_persistentvolumeclaim_resource_requests_storage_bytes"
	monitoringMetricKubeDaemonSetMetadataGeneration                       = "kube_daemonset_metadata_generation"
	monitoringMetricKubeDaemonSetStatusCurrentNumberScheduled             = "kube_daemonset_status_current_number_scheduled"
	monitoringMetricKubeDaemonSetStatusDesiredNumberScheduled             = "kube_daemonset_status_desired_number_scheduled"
	monitoringMetricKubeDaemonSetStatusNumberAvailable                    = "kube_daemonset_status_number_available"
	monitoringMetricKubeDaemonSetStatusNumberUnavailable                  = "kube_daemonset_status_number_unavailable"
	monitoringMetricKubeDaemonSetStatusUpdatedNumberScheduled             = "kube_daemonset_status_updated_number_scheduled"
	monitoringMetricKubeDeploymentMetadataGeneration                      = "kube_deployment_metadata_generation"
	monitoringMetricKubeDeploymentSpecReplicas                            = "kube_deployment_spec_replicas"
	monitoringMetricKubeDeploymentStatusObservedGeneration                = "kube_deployment_status_observed_generation"
	monitoringMetricKubeDeploymentStatusReplicas                          = "kube_deployment_status_replicas"
	monitoringMetricKubeDeploymentStatusReplicasAvailable                 = "kube_deployment_status_replicas_available"
	monitoringMetricKubeDeploymentStatusReplicasUnavailable               = "kube_deployment_status_replicas_unavailable"
	monitoringMetricKubeDeploymentStatusReplicasUpdated                   = "kube_deployment_status_replicas_updated"
	monitoringMetricKubeHorizontalPodAutoscalerSpecMaxReplicas            = "kube_horizontalpodautoscaler_spec_max_replicas"
	monitoringMetricKubeHorizontalPodAutoscalerSpecMinReplicas            = "kube_horizontalpodautoscaler_spec_min_replicas"
	monitoringMetricKubeHorizontalPodAutoscalerStatusCurrentReplicas      = "kube_horizontalpodautoscaler_status_current_replicas"
	monitoringMetricKubeHorizontalPodAutoscalerStatusDesiredReplicas      = "kube_horizontalpodautoscaler_status_desired_replicas"
	monitoringMetricKubeHorizontalPodAutoscalerStatusCondition            = "kube_horizontalpodautoscaler_status_condition"
	monitoringMetricKubeNodeInfo                                          = "kube_node_info"
	monitoringMetricKubeNodeLabels                                        = "kube_node_labels"
	monitoringMetricKubeNodeSpecUnschedulable                             = "kube_node_spec_unschedulable"
	monitoringMetricKubeNodeStatusAllocatable                             = "kube_node_status_allocatable"
	monitoringMetricKubeNodeStatusCapacity                                = "kube_node_status_capacity"
	monitoringMetricKubeNodeStatusCondition                               = "kube_node_status_condition"
	monitoringMetricKubePodContainer_info                                 = "kube_pod_container_info"
	monitoringMetricKubePodContainerResourceLimits                        = "kube_pod_container_resource_limits"
	monitoringMetricKubePodContainerResourceRequests                      = "kube_pod_container_resource_requests"
	monitoringMetricKubePodContainerStatusRestartsTotal                   = "kube_pod_container_status_restarts_total"
	monitoringMetricKubePodInfo                                           = "kube_pod_info"
	monitoringMetricKubePodLabels                                         = "kube_pod_labels"
	monitoringMetricKubePodStatusPhase                                    = "kube_pod_status_phase"
	monitoringMetricKubePodStatusReady                                    = "kube_pod_status_ready"
	monitoringMetricKubeStatefulSetMetadataGeneration                     = "kube_statefulset_metadata_generation"
	monitoringMetricKubeStatefulSetReplicas                               = "kube_statefulset_replicas"
	monitoringMetricKubeStatefulSetStatusObservedGeneration               = "kube_statefulset_status_observed_generation"
	monitoringMetricKubeStatefulSetStatusReplicas                         = "kube_statefulset_status_replicas"
	monitoringMetricKubeStatefulSetStatusReplicasCurrent                  = "kube_statefulset_status_replicas_current"
	monitoringMetricKubeStatefulSetStatusReplicasReady                    = "kube_statefulset_status_replicas_ready"
	monitoringMetricKubeStatefulSetStatusReplicasUpdated                  = "kube_statefulset_status_replicas_updated"
)

var (
	centralMonitoringAllowedMetrics = []string{
		monitoringMetricKubePersistentVolumeClaimResourceRequestsStorageBytes,
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
		monitoringMetricKubePodContainer_info,
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

	centralMonitoringScrapeConfig = `job_name: ` + monitoringPrometheusJobName + `
honor_labels: false
# Service is used, because we only care about metric from one kube-state-metrics instance
# and not multiple in HA setup
kubernetes_sd_configs:
- role: service
  namespaces:
    names: [ ` + v1beta1constants.GardenNamespace + ` ]
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
- source_labels: [ pod ]
  regex: ^.+\.tf-pod.+$
  action: drop
- source_labels: [ __name__ ]
  action: keep
  regex: ^(` + strings.Join(centralMonitoringAllowedMetrics, "|") + `)$
`
)

// CentralMonitoringConfiguration returns scrape configs for the central Prometheus.
func CentralMonitoringConfiguration() (component.CentralMonitoringConfig, error) {
	return component.CentralMonitoringConfig{ScrapeConfigs: []string{centralMonitoringScrapeConfig}}, nil
}
