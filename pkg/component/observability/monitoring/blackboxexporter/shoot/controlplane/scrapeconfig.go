// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"k8s.io/utils/ptr"

	shootprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

// ScrapeConfig returns the scrape configs related to the blackbox-exporter for the shoot control plane use-case.
func ScrapeConfig(namespace string, kubeAPIServerTarget monitoringv1alpha1.Target) []*monitoringv1alpha1.ScrapeConfig {
	return []*monitoringv1alpha1.ScrapeConfig{{
		ObjectMeta: monitoringutils.ConfigObjectMeta("blackbox-apiserver", namespace, shootprometheus.Label),
		Spec: monitoringv1alpha1.ScrapeConfigSpec{
			Params:      map[string][]string{"module": {moduleName}},
			MetricsPath: ptr.To("/probe"),
			StaticConfigs: []monitoringv1alpha1.StaticConfig{{
				Targets: []monitoringv1alpha1.Target{kubeAPIServerTarget},
			}},
			RelabelConfigs: []monitoringv1.RelabelConfig{
				{
					TargetLabel: "type",
					Replacement: ptr.To("seed"),
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__address__"},
					TargetLabel:  "__param_target",
					Action:       "replace",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__param_target"},
					TargetLabel:  "instance",
					Action:       "replace",
				},
				{
					TargetLabel: "__address__",
					Replacement: ptr.To("blackbox-exporter:9115"),
					Action:      "replace",
				},
				{
					Action:      "replace",
					Replacement: ptr.To("blackbox-apiserver"),
					TargetLabel: "job",
				},
			},
		},
	}}
}
