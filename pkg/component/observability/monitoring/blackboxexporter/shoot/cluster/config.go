// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"time"

	blackboxexporterconfig "github.com/prometheus/blackbox_exporter/config"
	prometheuscommonconfig "github.com/prometheus/common/config"
)

// Config returns the blackbox-exporter config for the shoot cluster use-case.
func Config() blackboxexporterconfig.Config {
	return blackboxexporterconfig.Config{Modules: map[string]blackboxexporterconfig.Module{
		"http_kubernetes_service": {
			Prober:  "http",
			Timeout: 10 * time.Second,
			HTTP: blackboxexporterconfig.HTTPProbe{
				Headers: map[string]string{
					"Accept":          "*/*",
					"Accept-Language": "en-US",
				},
				HTTPClientConfig: prometheuscommonconfig.HTTPClientConfig{
					TLSConfig:       prometheuscommonconfig.TLSConfig{CAFile: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"},
					BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
				},
				IPProtocol: "ipv4",
			},
		},
	}}
}
