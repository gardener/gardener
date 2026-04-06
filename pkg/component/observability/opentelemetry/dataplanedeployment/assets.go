// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dataplanedeployment

import (
	_ "embed"
)

// otelConfig contains the complete OpenTelemetry collector configuration
// for scraping kubelet, cadvisor, and node-exporter metrics in the shoot data plane.
//
//go:embed assets/opentelemetry-collector-dataplane-config.yaml
var otelConfig string
