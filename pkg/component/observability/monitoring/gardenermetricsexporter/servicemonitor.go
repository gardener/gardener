// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenermetricsexporter

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

func (g *gardenerMetricsExporter) serviceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta(deploymentName, g.namespace, garden.Label),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: GetLabels()},
			Endpoints: []monitoringv1.Endpoint{{
				TargetPort: ptr.To(intstr.FromInt32(probePort)),
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"garden_projects_status",
					"garden_users_total",
					"garden_shoot_info",
					"garden_shoot_condition",
					"garden_shoot_node_info",
					"garden_shoot_operation_states",
					"garden_shoot_node_max_total",
					"garden_shoot_node_min_total",
					"garden_shoot_response_duration_milliseconds",
					"garden_shoot_operations_total",
					"garden_shoots_hibernation_enabled_total",
					"garden_shoots_hibernation_schedule_total",
					"garden_shoot_hibernated",
					"garden_shoots_custom_addon_kubedashboard_total",
					"garden_shoots_custom_addon_nginxingress_total",
					"garden_shoots_custom_apiserver_auditpolicy_total",
					"garden_shoots_custom_apiserver_basicauth_total",
					"garden_shoots_custom_apiserver_featuregates_total",
					"garden_shoots_custom_apiserver_oidcconfig_total",
					"garden_shoots_custom_extensions_total",
					"garden_shoots_custom_kcm_horizontalpodautoscale_total",
					"garden_shoots_custom_kcm_nodecidrmasksize_total",
					"garden_shoots_custom_kubelet_podpidlimit_total",
					"garden_shoots_custom_network_customdomain_total",
					"garden_shoots_custom_proxy_mode_total",
					"garden_shoots_custom_worker_annotations_total",
					"garden_shoots_custom_worker_multiplepools_total",
					"garden_shoots_custom_worker_multizones_total",
					"garden_shoots_custom_worker_taints_total",
					"garden_seed_info",
					"garden_seed_condition",
					"garden_seed_capacity",
					"garden_seed_usage",
				),
			}},
		},
	}
}
