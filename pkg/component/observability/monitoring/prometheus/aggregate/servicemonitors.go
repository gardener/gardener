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

package aggregate

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// CentralServiceMonitors returns the central ServiceMonitor resources for the aggregate prometheus.
func CentralServiceMonitors() []*monitoringv1.ServiceMonitor {
	return []*monitoringv1.ServiceMonitor{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "shoot-prometheus"},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: map[string]string{
					v1beta1constants.LabelApp:  "prometheus",
					v1beta1constants.LabelRole: v1beta1constants.LabelMonitoring,
				}},
				NamespaceSelector: monitoringv1.NamespaceSelector{Any: true},
				Endpoints: []monitoringv1.Endpoint{{
					Path:            "/federate",
					HonorTimestamps: ptr.To(false),
					HonorLabels:     true,
					Params: map[string][]string{
						"match[]": {
							`{__name__="shoot:availability"}`,
							`{__name__=~"shoot:(.+):(.+)"}`,
							`{__name__="ALERTS"}`,
							`{__name__="prometheus_tsdb_lowest_timestamp"}`,
							`{__name__="prometheus_tsdb_storage_blocks_bytes"}`,
							`{__name__="kubeproxy_network_latency:quantile"}`,
							`{__name__="kubeproxy_sync_proxy:quantile"}`,
						},
					},
					Port: "metrics",
					RelabelConfigs: []*monitoringv1.RelabelConfig{
						// This service monitor is targeting the prometheis in multiple namespaces. Without explicitly
						// overriding the job label, prometheus-operator would choose job=prometheus-web (service name).
						{
							Action:      "replace",
							Replacement: "shoot-prometheus",
							TargetLabel: "job",
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_namespace"},
							Regex:        `shoot-(.+)`,
							Action:       "keep",
						},
					},
				}},
			},
		},
	}
}
