// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"bytes"
	_ "embed"
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
	IsManagedSeed bool
}

// AdditionalScrapeConfigs returns the additional scrape configs for the cache prometheus.
func AdditionalScrapeConfigs(isManagedSeed bool) []string {
	return []string{
		process(cAdvisor, isManagedSeed),
		process(kubelet, isManagedSeed),
	}
}

func process(text string, isManagedSeed bool) string {
	data := Data{
		IsManagedSeed: isManagedSeed,
	}

	tmpl, err := template.New("Template").Parse(text)
	if err != nil {
		panic(err)
	}

	var result bytes.Buffer
	if err := tmpl.Execute(&result, data); err != nil {
		panic(err)
	}

	return result.String()
}
