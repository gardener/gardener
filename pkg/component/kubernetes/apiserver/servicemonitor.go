// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
)

func (k *kubeAPIServer) emptyServiceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{ObjectMeta: monitoringutils.ConfigObjectMeta(k.values.NamePrefix+v1beta1constants.DeploymentNameKubeAPIServer, k.namespace, k.prometheusLabel())}
}

func (k *kubeAPIServer) reconcileServiceMonitor(ctx context.Context, serviceMonitor *monitoringv1.ServiceMonitor) error {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), serviceMonitor, func() error {
		serviceMonitor.Labels = monitoringutils.Labels(k.prometheusLabel())
		serviceMonitor.Spec = monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: getLabels()},
			Endpoints: []monitoringv1.Endpoint{{
				TargetPort: ptr.To(intstr.FromInt32(kubeapiserverconstants.Port)),
				Scheme:     "https",
				TLSConfig:  &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)}},
				Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: k.prometheusAccessSecretName()},
					Key:                  resourcesv1alpha1.DataKeyToken,
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{{
					Action: "labelmap",
					Regex:  `__meta_kubernetes_service_label_(.+)`,
				}},
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"apiserver_admission_controller_admission_duration_seconds_.+",
					"apiserver_admission_webhook_admission_duration_seconds_.+",
					"apiserver_admission_step_admission_duration_seconds_.+",
					"apiserver_admission_webhook_request_total",
					"apiserver_admission_webhook_rejection_count",
					"apiserver_audit_event_total",
					"apiserver_audit_error_total",
					"apiserver_audit_requests_rejected_total",
					"apiserver_cache_list_.+",
					"apiserver_crd_webhook_conversion_duration_seconds_.+",
					"apiserver_current_inflight_requests",
					"apiserver_current_inqueue_requests",
					"apiserver_init_events_total",
					"apiserver_latency",
					"apiserver_latency_seconds",
					"apiserver_longrunning_requests",
					"apiserver_request_duration_seconds_.+",
					"apiserver_request_duration_seconds_bucket",
					"apiserver_request_duration_seconds_count",
					"apiserver_request_terminations_total",
					"apiserver_response_sizes_.+",
					// TODO: The apiserver_storage_db_total_size_in_bytes metric is replaced by
					//  apiserver_storage_size_bytes in K8s 1.28 (https://github.com/kubernetes/kubernetes/pull/118812).
					//  Remove this metric when we remove support for K8s 1.28.
					"apiserver_storage_db_total_size_in_bytes",
					"apiserver_storage_list_.+",
					"apiserver_storage_objects",
					"apiserver_storage_transformation_duration_seconds_.+",
					"apiserver_storage_transformation_operations_total",
					"apiserver_storage_size_bytes",
					"apiserver_registered_watchers",
					"apiserver_request_count",
					"apiserver_request_total",
					"apiserver_watch_duration",
					"apiserver_watch_events_sizes_.+",
					"apiserver_watch_events_total",
					"etcd_request_duration_seconds_.+",
					"go_.+",
					"process_max_fds",
					"process_open_fds",
					"watch_cache_capacity_increase_total",
					"watch_cache_capacity_decrease_total",
					"watch_cache_capacity",
				),
			}},
		}
		return nil
	})
	return err
}
