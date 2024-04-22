// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garden

import (
	"time"

	blackboxexporterconfig "github.com/prometheus/blackbox_exporter/config"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const (
	httpGardenerAPIServerModuleName    = "http_gardener_apiserver"
	httpKubeAPIServerModuleName        = "http_kube_apiserver"
	httpKubeAPIServerRootCAsModuleName = "http_kube_apiserver_root_cas"
	httpGardenerDashboardModuleName    = "http_gardener_dashboard"
)

// Config returns the blackbox-exporter config for the garden use-case.
func Config() blackboxexporterconfig.Config {
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

		httpGardenerAPIServerModule    = defaultModuleConfig()
		httpKubeAPIServerModule        = defaultModuleConfig()
		httpKubeAPIServerRootCAsModule = defaultModuleConfig()
		httpGardenerDashboardModule    = defaultModuleConfig()

		pathGardenerAPIServerCABundle = blackboxexporter.VolumeMountPathGardenerCA + "/" + secretsutils.DataKeyCertificateBundle
		pathKubeAPIServerCABundle     = blackboxexporter.VolumeMountPathClusterAccess + "/" + secretsutils.DataKeyCertificateBundle
		pathToken                     = blackboxexporter.VolumeMountPathClusterAccess + "/" + resourcesv1alpha1.DataKeyToken
	)

	httpGardenerAPIServerModule.HTTP.HTTPClientConfig.TLSConfig.CAFile = pathGardenerAPIServerCABundle
	httpKubeAPIServerModule.HTTP.HTTPClientConfig.TLSConfig.CAFile = pathKubeAPIServerCABundle
	httpKubeAPIServerModule.HTTP.HTTPClientConfig.BearerTokenFile = pathToken
	httpKubeAPIServerRootCAsModule.HTTP.HTTPClientConfig.BearerTokenFile = pathToken

	return blackboxexporterconfig.Config{Modules: map[string]blackboxexporterconfig.Module{
		httpGardenerAPIServerModuleName:    httpGardenerAPIServerModule,
		httpKubeAPIServerModuleName:        httpKubeAPIServerModule,
		httpKubeAPIServerRootCAsModuleName: httpKubeAPIServerRootCAsModule,
		httpGardenerDashboardModuleName:    httpGardenerDashboardModule,
	}}
}
