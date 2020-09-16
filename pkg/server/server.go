// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gardener/gardener/pkg/logger"
)

// Server is a HTTP(S) server.
type Server struct {
	bindAddress  string
	port         int
	tlsCertPath  *string
	tlsKeyPath   *string
	handlers     map[string]http.Handler
	handlerFuncs map[string]http.HandlerFunc
}

// Start starts the server. If the TLS cert and key paths are provided then it will start it as HTTPS server.
func (s *Server) Start(ctx context.Context) {
	var (
		listenAddress = fmt.Sprintf("%s:%d", s.bindAddress, s.port)
		serverMux     = http.NewServeMux()
		server        = &http.Server{Addr: listenAddress, Handler: serverMux}
	)

	// Add handlers to HTTPS server and start it.
	for pattern, handler := range s.handlers {
		serverMux.Handle(pattern, handler)
	}
	for pattern, handlerFunc := range s.handlerFuncs {
		serverMux.HandleFunc(pattern, handlerFunc)
	}

	// Server startup logic.
	go func() {
		if s.tlsCertPath != nil && s.tlsKeyPath != nil {
			logger.Logger.Infof("Starting new HTTPS server on %s", listenAddress)
			if err := server.ListenAndServeTLS(*s.tlsCertPath, *s.tlsKeyPath); err != http.ErrServerClosed {
				logger.Logger.Errorf("Could not start HTTPS server: %v", err)
			}
			return
		}

		logger.Logger.Infof("Starting new HTTP server on %s", listenAddress)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Logger.Errorf("Could not start HTTP server: %v", err)
		}
	}()

	// Server shutdown logic.
	<-ctx.Done()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Logger.Errorf("Error when shutting down server: %v", err)
	}
	logger.Logger.Info("Server stopped.")
}
