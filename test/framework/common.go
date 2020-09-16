// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import "time"

const (
	k8sClientInitPollInterval = 20 * time.Second
	k8sClientInitTimeout      = 5 * time.Minute
	defaultPollInterval       = 5 * time.Second

	// KubeconfigSecretKeyName ist the name of the key in a secret that holds the kubeconfig of a shoot
	KubeconfigSecretKeyName = "kubeconfig"

	// LoggingUserName is the admin user name for the loki instance of a shoot
	lokiLogging = "loki"
	lokiPort    = 3100

	// IntegrationTestPrefix is the default prefix that will be used for test shoots if none other is specified
	IntegrationTestPrefix = "itest-"

	// WorkerNamePrefix is the default prefix that will be used for Shoot workers
	WorkerNamePrefix = "worker-"
)

// SearchResponse represents the response from a search query to loki
type SearchResponse struct {
	Data struct {
		Stats struct {
			Summary struct {
				TotalLinesProcessed int `json:"totalLinesProcessed"`
			} `json:"summary"`
		} `json:"stats"`
	} `json:"data"`
}
