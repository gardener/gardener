// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	blackboxexporterconfig "github.com/prometheus/blackbox_exporter/config"
	prometheuscommonconfig "github.com/prometheus/common/config"

	. "github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter/garden"
)

var _ = Describe("Config", func() {
	Describe("#Config", func() {
		It("should return the expected config for the garden's blackbox-exporter", func() {
			Expect(Config()).To(Equal(blackboxexporterconfig.Config{Modules: map[string]blackboxexporterconfig.Module{
				"http_gardener_apiserver": {
					Prober:  "http",
					Timeout: 10 * time.Second,
					HTTP: blackboxexporterconfig.HTTPProbe{
						Headers: map[string]string{
							"Accept":          "*/*",
							"Accept-Language": "en-US",
						},
						HTTPClientConfig: prometheuscommonconfig.HTTPClientConfig{
							TLSConfig: prometheuscommonconfig.TLSConfig{CAFile: "/var/run/secrets/blackbox_exporter/gardener-ca/bundle.crt"},
						},
						IPProtocol: "ipv4",
					},
				},
				"http_kube_apiserver": {
					Prober:  "http",
					Timeout: 10 * time.Second,
					HTTP: blackboxexporterconfig.HTTPProbe{
						Headers: map[string]string{
							"Accept":          "*/*",
							"Accept-Language": "en-US",
						},
						HTTPClientConfig: prometheuscommonconfig.HTTPClientConfig{
							TLSConfig:       prometheuscommonconfig.TLSConfig{CAFile: "/var/run/secrets/blackbox_exporter/cluster-access/bundle.crt"},
							BearerTokenFile: "/var/run/secrets/blackbox_exporter/cluster-access/token",
						},
						IPProtocol: "ipv4",
					},
				},
				"http_kube_apiserver_root_cas": {
					Prober:  "http",
					Timeout: 10 * time.Second,
					HTTP: blackboxexporterconfig.HTTPProbe{
						Headers: map[string]string{
							"Accept":          "*/*",
							"Accept-Language": "en-US",
						},
						HTTPClientConfig: prometheuscommonconfig.HTTPClientConfig{
							BearerTokenFile: "/var/run/secrets/blackbox_exporter/cluster-access/token",
						},
						IPProtocol: "ipv4",
					},
				},
				"http_gardener_dashboard": {
					Prober:  "http",
					Timeout: 10 * time.Second,
					HTTP: blackboxexporterconfig.HTTPProbe{
						Headers: map[string]string{
							"Accept":          "*/*",
							"Accept-Language": "en-US",
						},
						IPProtocol: "ipv4",
					},
				},
				"http_gardener_discovery_server": {
					Prober:  "http",
					Timeout: 10 * time.Second,
					HTTP: blackboxexporterconfig.HTTPProbe{
						Headers: map[string]string{
							"Accept":          "*/*",
							"Accept-Language": "en-US",
						},
						IPProtocol: "ipv4",
					},
				},
			}}))
		})
	})
})
