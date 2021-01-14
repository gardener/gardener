// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package webhook

import (
	"context"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

// remoteAddrContextKey is a context key. It will be filled by the remoteAddrInjectingHandler with the received
// request's RemoteAddr field value.
// The associated value will be of type string.
var remoteAddrContextKey = struct{}{}

// remoteAddrInjectingHandler is a wrapper around a given http.Handler that injects the requests.RemoteAddr into the
// request's context and delegates to the underlying handler.
type remoteAddrInjectingHandler struct {
	// Handler is the underlying handler.
	http.Handler
}

// InjectFunc injects into the underlying handler.
func (h remoteAddrInjectingHandler) InjectFunc(f inject.Func) error {
	return f(h.Handler)
}

// ServerHTTP implements http.Handler by delegating to the underlying handler but injecting request.RemoteAddr into
// the request's context.
func (h remoteAddrInjectingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Handler.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), remoteAddrContextKey, r.RemoteAddr)))
}
