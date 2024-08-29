// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"time"

	blackboxexporterconfig "github.com/prometheus/blackbox_exporter/config"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"net/http"
)

const (
	httpGardenerAPIServerModuleName       = "http_gardener_apiserver"
	httpKubeAPIServerModuleName           = "http_kube_apiserver"
	httpKubeAPIServerRootCAsModuleName    = "http_kube_apiserver_root_cas"
	httpGardenerDashboardModuleName       = "http_gardener_dashboard"
	httpGardenerDiscoveryServerModuleName = "http_gardener_discovery_server"

	// Name of the module to be used for scraping of webhooks
	HttpWebhookServerModuleName = "http_webhooks"
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

		// As we want to only check tls certificates for webhooks and don't care
		// for the rest of the response, all of the following should be treated as OK.
		// 5XX are excluded as they indicate that the returned result is not reliable
		webhookOKStatusCodes = []int{
			http.StatusOK,                           // 200
			http.StatusCreated,                      // 201
			http.StatusAccepted,                     // 202
			http.StatusNonAuthoritativeInfo,         // 203
			http.StatusNoContent,                    // 204
			http.StatusResetContent,                 // 205
			http.StatusPartialContent,               // 206
			http.StatusMultiStatus,                  // 207
			http.StatusAlreadyReported,              // 208
			http.StatusIMUsed,                       // 226
			http.StatusMultipleChoices,              // 300
			http.StatusMovedPermanently,             // 301
			http.StatusFound,                        // 302
			http.StatusSeeOther,                     // 303
			http.StatusNotModified,                  // 304
			http.StatusUseProxy,                     // 305
			http.StatusTemporaryRedirect,            // 307
			http.StatusPermanentRedirect,            // 308
			http.StatusBadRequest,                   // 400
			http.StatusUnauthorized,                 // 401
			http.StatusPaymentRequired,              // 402
			http.StatusForbidden,                    // 403
			http.StatusNotFound,                     // 404
			http.StatusMethodNotAllowed,             // 405
			http.StatusNotAcceptable,                // 406
			http.StatusProxyAuthRequired,            // 407
			http.StatusRequestTimeout,               // 408
			http.StatusConflict,                     // 409
			http.StatusGone,                         // 410
			http.StatusLengthRequired,               // 411
			http.StatusPreconditionFailed,           // 412
			http.StatusRequestEntityTooLarge,        // 413
			http.StatusRequestURITooLong,            // 414
			http.StatusUnsupportedMediaType,         // 415
			http.StatusRequestedRangeNotSatisfiable, // 416
			http.StatusExpectationFailed,            // 417
			http.StatusTeapot,                       // 418
			http.StatusMisdirectedRequest,           // 421
			http.StatusUnprocessableEntity,          // 422
			http.StatusLocked,                       // 423
			http.StatusFailedDependency,             // 424
			http.StatusUpgradeRequired,              // 426
			http.StatusPreconditionRequired,         // 428
			http.StatusTooManyRequests,              // 429
			http.StatusRequestHeaderFieldsTooLarge,  // 431
			http.StatusUnavailableForLegalReasons,   // 451
		}

		httpGardenerAPIServerModule       = defaultModuleConfig()
		httpKubeAPIServerModule           = defaultModuleConfig()
		httpKubeAPIServerRootCAsModule    = defaultModuleConfig()
		httpGardenerDashboardModule       = defaultModuleConfig()
		httpGardenerDiscoveryServerModule = defaultModuleConfig()
		httpWebhookServerModule           = defaultModuleConfig()

		pathGardenerAPIServerCABundle = blackboxexporter.VolumeMountPathGardenerCA + "/" + secretsutils.DataKeyCertificateBundle
		pathKubeAPIServerCABundle     = blackboxexporter.VolumeMountPathClusterAccess + "/" + secretsutils.DataKeyCertificateBundle
		pathToken                     = blackboxexporter.VolumeMountPathClusterAccess + "/" + resourcesv1alpha1.DataKeyToken
	)

	httpGardenerAPIServerModule.HTTP.HTTPClientConfig.TLSConfig.CAFile = pathGardenerAPIServerCABundle
	httpKubeAPIServerModule.HTTP.HTTPClientConfig.TLSConfig.CAFile = pathKubeAPIServerCABundle
	httpKubeAPIServerModule.HTTP.HTTPClientConfig.BearerTokenFile = pathToken
	httpKubeAPIServerRootCAsModule.HTTP.HTTPClientConfig.BearerTokenFile = pathToken
	// Webhooks are using this certificate as CA
	httpWebhookServerModule.HTTP.HTTPClientConfig.TLSConfig.CAFile = pathGardenerAPIServerCABundle
	httpWebhookServerModule.HTTP.ValidStatusCodes = webhookOKStatusCodes

	if isDashboardCertificateIssuedByGardener {
		httpGardenerDashboardModule.HTTP.HTTPClientConfig.TLSConfig.CAFile = pathGardenerAPIServerCABundle
	}

	config := blackboxexporterconfig.Config{Modules: map[string]blackboxexporterconfig.Module{
		httpGardenerAPIServerModuleName:    httpGardenerAPIServerModule,
		httpKubeAPIServerModuleName:        httpKubeAPIServerModule,
		httpKubeAPIServerRootCAsModuleName: httpKubeAPIServerRootCAsModule,
		httpGardenerDashboardModuleName:    httpGardenerDashboardModule,
		HttpWebhookServerModuleName:        httpWebhookServerModule,
	}}

	if isGardenerDiscoveryServerEnabled {
		config.Modules[httpGardenerDiscoveryServerModuleName] = httpGardenerDiscoveryServerModule
	}

	return config
}
