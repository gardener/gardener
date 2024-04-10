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

package garden

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"k8s.io/utils/ptr"

	gardenprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

// ScrapeConfig returns the scrape configs related to the blackbox-exporter for the garden use-case.
func ScrapeConfig(namespace string, kubeAPIServerTargets []monitoringv1alpha1.Target) []*monitoringv1alpha1.ScrapeConfig {
	defaultScrapeConfig := func(name, module string, targets []monitoringv1alpha1.Target) *monitoringv1alpha1.ScrapeConfig {
		return &monitoringv1alpha1.ScrapeConfig{
			ObjectMeta: monitoringutils.ConfigObjectMeta("blackbox-"+name, namespace, gardenprometheus.Label),
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				Params:      map[string][]string{"module": {module}},
				MetricsPath: ptr.To("/probe"),
				StaticConfigs: []monitoringv1alpha1.StaticConfig{{
					Targets: targets,
					Labels:  map[monitoringv1.LabelName]string{"purpose": "availability"},
				}},
				RelabelConfigs: []*monitoringv1.RelabelConfig{
					{
						SourceLabels: []monitoringv1.LabelName{"__address__"},
						Separator:    ptr.To(";"),
						Regex:        `(.*)`,
						TargetLabel:  "__param_target",
						Replacement:  `$1`,
						Action:       "replace",
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__param_target"},
						Separator:    ptr.To(";"),
						Regex:        `(.*)`,
						TargetLabel:  "instance",
						Replacement:  `$1`,
						Action:       "replace",
					},
					{
						Separator:   ptr.To(";"),
						Regex:       `(.*)`,
						TargetLabel: "__address__",
						Replacement: "blackbox-exporter:9115",
						Action:      "replace",
					},
					{
						Action:      "replace",
						Replacement: "blackbox-" + name,
						TargetLabel: "job",
					},
				},
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"probe_success",
					"probe_http_status_code",
					"probe_http_duration_seconds",
				),
			},
		}
	}

	var (
		gardenerAPIServerScrapeConfig = defaultScrapeConfig("gardener-apiserver", httpGardenerAPIServerModuleName, []monitoringv1alpha1.Target{"https://gardener-apiserver.garden.svc/healthz"})
		kubeAPIServerScrapeConfig     = defaultScrapeConfig("apiserver", httpKubeAPIServerModuleName, kubeAPIServerTargets)
	)

	kubeAPIServerScrapeConfig.Spec.RelabelConfigs = append([]*monitoringv1.RelabelConfig{{
		SourceLabels: []monitoringv1.LabelName{"__address__"},
		Separator:    ptr.To(";"),
		Regex:        `https://api\..*`,
		TargetLabel:  "__param_module",
		Replacement:  httpKubeAPIServerRootCAsModuleName,
		Action:       "replace",
	}}, kubeAPIServerScrapeConfig.Spec.RelabelConfigs...)

	return []*monitoringv1alpha1.ScrapeConfig{gardenerAPIServerScrapeConfig, kubeAPIServerScrapeConfig}
}
