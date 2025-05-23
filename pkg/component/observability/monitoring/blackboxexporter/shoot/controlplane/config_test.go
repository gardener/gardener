// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	blackboxexporterconfig "github.com/prometheus/blackbox_exporter/config"
	prometheuscommonconfig "github.com/prometheus/common/config"

	. "github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter/shoot/controlplane"
)

var _ = Describe("Config", func() {
	Describe("#Config", func() {
		It("should return the expected config for the shoot control plane's blackbox-exporter", func() {
			Expect(Config()).To(Equal(blackboxexporterconfig.Config{Modules: map[string]blackboxexporterconfig.Module{
				"http_apiserver": {
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
			}}))
		})
	})
})
