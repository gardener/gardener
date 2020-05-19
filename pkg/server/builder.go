// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"net/http"
	"sync"
)

// Builder is a new builder for Servers.
type Builder struct {
	bindAddress  string
	port         int
	tlsCertPath  *string
	tlsKeyPath   *string
	handlers     map[string]http.Handler
	handlerFuncs map[string]http.HandlerFunc

	lock sync.Mutex
}

// NewBuilder creates a new builder object for servers.
func NewBuilder() *Builder {
	return &Builder{
		bindAddress:  "0.0.0.0",
		port:         8080,
		handlers:     make(map[string]http.Handler),
		handlerFuncs: make(map[string]http.HandlerFunc),
	}
}

// WithBindAddress sets the bind address.
func (b *Builder) WithBindAddress(bindAddress string) *Builder {
	b.bindAddress = bindAddress
	return b
}

// WithPort sets the port.
func (b *Builder) WithPort(port int) *Builder {
	b.port = port
	return b
}

// WithTLS sets the paths for the TLS certificate and key. If  they are set then a HTTPS server will be built.
func (b *Builder) WithTLS(certPath, keyPath string) *Builder {
	b.tlsCertPath = &certPath
	b.tlsKeyPath = &keyPath
	return b
}

// WithHandlers sets the handlers list.
func (b *Builder) WithHandlers(handlers map[string]http.Handler) *Builder {
	b.handlers = handlers
	return b
}

// WithHandler adds a specific handler.
func (b *Builder) WithHandler(pattern string, handler http.Handler) *Builder {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.handlers[pattern] = handler
	return b
}

// WithHandlerFuncs sets the handlerFuncs list.
func (b *Builder) WithHandlerFuncs(handlerFuncs map[string]http.HandlerFunc) *Builder {
	b.handlerFuncs = handlerFuncs
	return b
}

// WithHandlerFunc adds a specific handlerFunc.
func (b *Builder) WithHandlerFunc(pattern string, handlerFunc http.HandlerFunc) *Builder {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.handlerFuncs[pattern] = handlerFunc
	return b
}

// Build builds a Server object.
func (b *Builder) Build() *Server {
	return &Server{
		bindAddress:  b.bindAddress,
		port:         b.port,
		tlsCertPath:  b.tlsCertPath,
		tlsKeyPath:   b.tlsKeyPath,
		handlers:     b.handlers,
		handlerFuncs: b.handlerFuncs,
	}
}
