// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gardener/gardener/pkg/apis/componentconfig"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/server/handlers"
	"github.com/gardener/gardener/pkg/server/handlers/webhooks"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/tools/cache"
)

// Serve starts a HTTP and a HTTPS server.
func Serve(ctx context.Context, k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.SharedInformerFactory, serverConfig componentconfig.ServerConfiguration) {
	var (
		listenAddressHTTP  = fmt.Sprintf("%s:%d", serverConfig.HTTP.BindAddress, serverConfig.HTTP.Port)
		listenAddressHTTPS = fmt.Sprintf("%s:%d", serverConfig.HTTPS.BindAddress, serverConfig.HTTPS.Port)

		serverMuxHTTP  = http.NewServeMux()
		serverMuxHTTPS = http.NewServeMux()

		serverHTTP  = &http.Server{Addr: listenAddressHTTP, Handler: serverMuxHTTP}
		serverHTTPS = &http.Server{Addr: listenAddressHTTPS, Handler: serverMuxHTTPS}
	)

	// Add handlers to HTTP server and start it.
	serverMuxHTTP.Handle("/metrics", promhttp.Handler())
	serverMuxHTTP.HandleFunc("/healthz", handlers.Healthz)

	go func() {
		logger.Logger.Infof("Starting HTTP server on %s", listenAddressHTTP)
		if err := serverHTTP.ListenAndServe(); err != http.ErrServerClosed {
			logger.Logger.Errorf("Could not start HTTP server: %v", err)
		}
	}()

	projectInformer := k8sGardenInformers.Garden().V1beta1().Projects()
	k8sGardenInformers.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), projectInformer.Informer().HasSynced) {
		panic("Timed out waiting for Garden caches to sync")
	}

	// Add handlers to HTTPS server and start it.
	serverMuxHTTPS.HandleFunc("/webhooks/validate-namespace-deletion", webhooks.NewValidateNamespaceDeletionHandler(k8sGardenClient, projectInformer.Lister()))

	go func() {
		logger.Logger.Infof("Starting HTTPS server on %s", listenAddressHTTPS)
		if err := serverHTTPS.ListenAndServeTLS(serverConfig.HTTPS.TLS.ServerCertPath, serverConfig.HTTPS.TLS.ServerKeyPath); err != http.ErrServerClosed {
			logger.Logger.Errorf("Could not start HTTPS server: %v", err)
		}
	}()

	// Server shutdown logic.
	<-ctx.Done()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := serverHTTP.Shutdown(ctx); err != nil {
		logger.Logger.Errorf("Error when shutting down HTTP server: %v", err)
	}
	if err := serverHTTPS.Shutdown(ctx); err != nil {
		logger.Logger.Errorf("Error when shutting down HTTPS server: %v", err)
	}
	logger.Logger.Info("HTTP(S) servers stopped.")
}
