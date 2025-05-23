// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"net/http"
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

// ServerHTTP implements http.Handler by delegating to the underlying handler but injecting request.RemoteAddr into
// the request's context.
func (h remoteAddrInjectingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Handler.ServeHTTP(w, r.Clone(context.WithValue(r.Context(), remoteAddrContextKey, r.RemoteAddr))) //nolint:staticcheck
}
