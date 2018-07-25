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

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/server/handlers"
)

// Serve starts a HTTP server.
func Serve(k8sGardenClient kubernetes.Client, bindAddress string, port int, metricsInterval time.Duration, stopCh chan struct{}) {
	http.Handle("/metrics", handlers.InitMetrics(k8sGardenClient, metricsInterval))
	http.HandleFunc("/healthz", handlers.Healthz)

	listenAddress := fmt.Sprintf("%s:%d", bindAddress, port)
	server := http.Server{
		Addr: listenAddress,
	}

	go func() {
		<-stopCh
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	go server.ListenAndServe()
	logger.Logger.Infof("Gardener controller manager HTTP server started (serving on %s)", listenAddress)
}
