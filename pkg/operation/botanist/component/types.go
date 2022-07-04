// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package component

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
	// Filters contains the filters for specific component.
	Filters string
	// Parser contains the parsers for specific component.
	Parsers string
	// UserExposed defines if the component is exposed to the end-user.
	UserExposed bool
	// PodPrefixes is the list of prefixes of the pod names when logging config is user-exposed.
	PodPrefixes []string
}
