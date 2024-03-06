// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
)

func (k *kubeAPIServer) emptyServiceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{ObjectMeta: monitoringutils.ConfigObjectMeta(k.values.NamePrefix+v1beta1constants.DeploymentNameKubeAPIServer, k.namespace, garden.Label)}
}

func (k *kubeAPIServer) reconcileServiceMonitor(ctx context.Context, serviceMonitor *monitoringv1.ServiceMonitor) error {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), serviceMonitor, func() error {
		serviceMonitor.Labels = monitoringutils.Labels(garden.Label)
		serviceMonitor.Spec = monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: getLabels()},
			Endpoints: []monitoringv1.Endpoint{{
				TargetPort: ptr.To(intstr.FromInt32(kubeapiserverconstants.Port)),
				Scheme:     "https",
				TLSConfig:  &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{InsecureSkipVerify: true}},
				Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: garden.AccessSecretName},
					Key:                  resourcesv1alpha1.DataKeyToken,
				}},
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"authentication_attempts",
					"authenticated_user_requests",
					"apiserver_admission_controller_admission_duration_seconds_.+",
					"apiserver_admission_webhook_admission_duration_seconds_.+",
					"apiserver_admission_step_admission_duration_seconds_.+",
					"apiserver_admission_webhook_rejection_count",
					"apiserver_audit_event_total",
					"apiserver_audit_error_total",
					"apiserver_audit_requests_rejected_total",
					"apiserver_request_total",
					"apiserver_latency_seconds",
					"apiserver_current_inflight_requests",
					"apiserver_current_inqueue_requests",
					"apiserver_response_sizes_.+",
					"apiserver_request_duration_seconds_.+",
					"apiserver_request_terminations_total",
					"apiserver_storage_objects",
					"apiserver_storage_transformation_duration_seconds_.+",
					"apiserver_storage_transformation_operations_total",
					"apiserver_registered_watchers",
					"apiserver_init_events_total",
					"apiserver_watch_events_sizes_.+",
					"apiserver_watch_events_total",
					"etcd_db_total_size_in_bytes",
					"etcd_request_duration_seconds_.+",
					"watch_cache_capacity_increase_total",
					"watch_cache_capacity_decrease_total",
					"watch_cache_capacity",
					"go_.+",
				),
			}},
		}
		return nil
	})
	return err
}
