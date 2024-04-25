// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package component

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
)

// Secret is a structure that contains information about a Kubernetes secret which is managed externally.
type Secret struct {
	// Name is the name of the Kubernetes secret object.
	Name string
	// Checksum is the checksum of the secret's data.
	Checksum string
	// Data is the data of the secret.
	Data map[string][]byte
}

// AggregateMonitoringConfig is a structure that contains configuration for the aggregate monitoring stack.
type AggregateMonitoringConfig struct {
	// ScrapeConfigs are the scrape configurations for aggregate Prometheus.
	ScrapeConfigs []string
}

// CentralMonitoringConfig is a structure that contains configuration for the central monitoring stack.
type CentralMonitoringConfig struct {
	// ScrapeConfigs are the scrape configurations for central Prometheus.
	ScrapeConfigs []string
	// CAdvisorScrapeConfigMetricRelabelConfigs are metric_relabel_configs for the cadvisor scrape config job.
	CAdvisorScrapeConfigMetricRelabelConfigs []string
}

// CentralLoggingConfig is a structure that contains configuration for the central logging stack.
type CentralLoggingConfig struct {
	// Inputs contains the inputs for specific component.
	Inputs []*fluentbitv1alpha2.ClusterInput
	// Filters contains the filters for specific component.
	Filters []*fluentbitv1alpha2.ClusterFilter
	// Parser contains the parsers for specific component.
	Parsers []*fluentbitv1alpha2.ClusterParser
}
