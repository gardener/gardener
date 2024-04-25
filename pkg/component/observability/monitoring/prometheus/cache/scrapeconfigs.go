// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	_ "embed"
)

var (
	//go:embed assets/scrapeconfigs/cadvisor.yaml
	cAdvisor string
	//go:embed assets/scrapeconfigs/kubelet.yaml
	kubelet string
)

// AdditionalScrapeConfigs returns the additional scrape configs for the cache prometheus.
func AdditionalScrapeConfigs() []string {
	return []string{
		cAdvisor,
		kubelet,
	}
}
