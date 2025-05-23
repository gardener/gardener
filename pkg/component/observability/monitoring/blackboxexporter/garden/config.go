// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"time"

	blackboxexporterconfig "github.com/prometheus/blackbox_exporter/config"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	httpGardenerAPIServerModuleName       = "http_gardener_apiserver"
	httpKubeAPIServerModuleName           = "http_kube_apiserver"
	httpKubeAPIServerRootCAsModuleName    = "http_kube_apiserver_root_cas"
	httpGardenerDashboardModuleName       = "http_gardener_dashboard"
	httpGardenerDiscoveryServerModuleName = "http_gardener_discovery_server"
)

// Config returns the blackbox-exporter config for the garden use-case.
func Config(isDashboardCertificateIssuedByGardener, isGardenerDiscoveryServerEnabled bool) blackboxexporterconfig.Config {
	var (
		defaultModuleConfig = func() blackboxexporterconfig.Module {
			return blackboxexporterconfig.Module{
				Prober:  "http",
				Timeout: 10 * time.Second,
				HTTP: blackboxexporterconfig.HTTPProbe{
					Headers: map[string]string{
						"Accept":          "*/*",
						"Accept-Language": "en-US",
					},
					IPProtocol: "ipv4",
				},
			}
		}

		httpGardenerAPIServerModule       = defaultModuleConfig()
		httpKubeAPIServerModule           = defaultModuleConfig()
		httpKubeAPIServerRootCAsModule    = defaultModuleConfig()
		httpGardenerDashboardModule       = defaultModuleConfig()
		httpGardenerDiscoveryServerModule = defaultModuleConfig()

		pathGardenerAPIServerCABundle = blackboxexporter.VolumeMountPathGardenerCA + "/" + secretsutils.DataKeyCertificateBundle
		pathKubeAPIServerCABundle     = blackboxexporter.VolumeMountPathClusterAccess + "/" + secretsutils.DataKeyCertificateBundle
		pathToken                     = blackboxexporter.VolumeMountPathClusterAccess + "/" + resourcesv1alpha1.DataKeyToken
	)

	httpGardenerAPIServerModule.HTTP.HTTPClientConfig.TLSConfig.CAFile = pathGardenerAPIServerCABundle
	httpKubeAPIServerModule.HTTP.HTTPClientConfig.TLSConfig.CAFile = pathKubeAPIServerCABundle
	httpKubeAPIServerModule.HTTP.HTTPClientConfig.BearerTokenFile = pathToken
	httpKubeAPIServerRootCAsModule.HTTP.HTTPClientConfig.BearerTokenFile = pathToken

	if isDashboardCertificateIssuedByGardener {
		httpGardenerDashboardModule.HTTP.HTTPClientConfig.TLSConfig.CAFile = pathGardenerAPIServerCABundle
	}

	config := blackboxexporterconfig.Config{Modules: map[string]blackboxexporterconfig.Module{
		httpGardenerAPIServerModuleName:    httpGardenerAPIServerModule,
		httpKubeAPIServerModuleName:        httpKubeAPIServerModule,
		httpKubeAPIServerRootCAsModuleName: httpKubeAPIServerRootCAsModule,
		httpGardenerDashboardModuleName:    httpGardenerDashboardModule,
	}}

	if isGardenerDiscoveryServerEnabled {
		config.Modules[httpGardenerDiscoveryServerModuleName] = httpGardenerDiscoveryServerModule
	}

	return config
}
