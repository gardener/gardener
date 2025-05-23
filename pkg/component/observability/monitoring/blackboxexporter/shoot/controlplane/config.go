// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"time"

	blackboxexporterconfig "github.com/prometheus/blackbox_exporter/config"
	prometheuscommonconfig "github.com/prometheus/common/config"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const moduleName = "http_apiserver"

// Config returns the blackbox-exporter config for the shoot control plane use-case.
func Config() blackboxexporterconfig.Config {
	return blackboxexporterconfig.Config{Modules: map[string]blackboxexporterconfig.Module{
		moduleName: {
			Prober:  "http",
			Timeout: 10 * time.Second,
			HTTP: blackboxexporterconfig.HTTPProbe{
				Headers: map[string]string{
					"Accept":          "*/*",
					"Accept-Language": "en-US",
				},
				HTTPClientConfig: prometheuscommonconfig.HTTPClientConfig{
					TLSConfig:       prometheuscommonconfig.TLSConfig{CAFile: blackboxexporter.VolumeMountPathClusterAccess + "/" + secretsutils.DataKeyCertificateBundle},
					BearerTokenFile: blackboxexporter.VolumeMountPathClusterAccess + "/" + resourcesv1alpha1.DataKeyToken,
				},
				IPProtocol: "ipv4",
			},
		},
	}}
}
