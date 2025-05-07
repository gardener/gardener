// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
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
