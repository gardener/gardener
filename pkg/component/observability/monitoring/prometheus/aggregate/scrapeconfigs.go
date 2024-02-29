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
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
)

// CentralScrapeConfigs returns the central ScrapeConfig resources for the aggregate prometheus.
func CentralScrapeConfigs() []*monitoringv1alpha1.ScrapeConfig {
	return []*monitoringv1alpha1.ScrapeConfig{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "prometheus"},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				HonorTimestamps: ptr.To(false),
				MetricsPath:     ptr.To("/federate"),
				Params: map[string][]string{
					"match[]": {
						`{__name__=~"metering:.+", __name__!~"metering:.+(over_time|_seconds|:this_month)"}`,
						`{__name__=~"seed:(.+):(.+)"}`,
						`{job="kube-state-metrics",namespace=~"garden|extension-.+"}`,
						`{job="kube-state-metrics",namespace=""}`,
						`{job="cadvisor",namespace=~"garden|extension-.+"}`,
					},
				},
				KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
					Role:       "service",
					Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{v1beta1constants.GardenNamespace}},
				}},
				RelabelConfigs: []*monitoringv1.RelabelConfig{
					{
						SourceLabels: []monitoringv1.LabelName{
							"__meta_kubernetes_service_name",
							"__meta_kubernetes_service_port_name",
						},
						Regex:  "prometheus-cache;" + prometheus.ServicePortName,
						Action: "keep",
					},
					{
						Action:      "replace",
						Replacement: "prometheus",
						TargetLabel: "job",
					},
				},
			},
		},
	}
}
