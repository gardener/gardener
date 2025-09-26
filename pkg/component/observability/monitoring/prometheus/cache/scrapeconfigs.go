// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var (
	//go:embed assets/scrapeconfigs/cadvisor.yaml
	cAdvisor string
	//go:embed assets/scrapeconfigs/kubelet.yaml
	kubelet string
)

// Data represents the data for the template.
type Data struct {
	SeedIsShoot bool
}

// CentralScrapeConfigs returns the central ScrapeConfig resources for the cache prometheus.
func CentralScrapeConfigs() []*monitoringv1alpha1.ScrapeConfig {
	return []*monitoringv1alpha1.ScrapeConfig{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prometheus-" + Label,
		},
		Spec: monitoringv1alpha1.ScrapeConfigSpec{
			RelabelConfigs: []monitoringv1.RelabelConfig{{
				Action:      "replace",
				Replacement: ptr.To("prometheus-" + Label),
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
func AdditionalScrapeConfigs(seedIsShoot bool) ([]string, error) {
	var out []string

	if result, err := process(cAdvisor, seedIsShoot); err != nil {
		return nil, fmt.Errorf("failed processing cadvisor scrape config template: %w", err)
	} else {
		out = append(out, result)
	}

	if result, err := process(kubelet, seedIsShoot); err != nil {
		return nil, fmt.Errorf("failed processing kubelet scrape config template: %w", err)
	} else {
		out = append(out, result)
	}

	return out, nil
}

func process(text string, seedIsShoot bool) (string, error) {
	data := Data{
		SeedIsShoot: seedIsShoot,
	}

	tmpl, err := template.New("Template").Parse(text)
	if err != nil {
		return "", fmt.Errorf("failed parsing template: %w", err)
	}

	var result bytes.Buffer
	if err := tmpl.Execute(&result, data); err != nil {
		return "", fmt.Errorf("failed rendering template: %w", err)
	}

	return result.String(), nil
}
