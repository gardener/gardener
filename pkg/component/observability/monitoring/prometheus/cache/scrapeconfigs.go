// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	//go:embed assets/scrapeconfigs/cadvisor.yaml
	cAdvisor string
	//go:embed assets/scrapeconfigs/kubelet.yaml
	kubelet string
)

// CentralScrapeConfigs returns the central ScrapeConfig resources for the cache prometheus.
func CentralScrapeConfigs() []*monitoringv1alpha1.ScrapeConfig {
	return []*monitoringv1alpha1.ScrapeConfig{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prometheus-" + Label,
		},
		Spec: monitoringv1alpha1.ScrapeConfigSpec{
			RelabelConfigs: []monitoringv1.RelabelConfig{{
				Action:      "replace",
				Replacement: new("prometheus-" + Label),
				TargetLabel: "job",
			}},
			StaticConfigs: []monitoringv1alpha1.StaticConfig{{
				Targets: []monitoringv1alpha1.Target{"localhost:9090"},
			}},
		},
	},
	}
}

// AdditionalScrapeConfigs returns the additional scrape configs for the cache prometheus.
func AdditionalScrapeConfigs() []string {
	return []string{cAdvisor, kubelet}
}
