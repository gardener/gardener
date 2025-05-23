// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"net/http"
	"net/http/pprof"
)

var (
	// ProfilingHandlers is list of profiling endpoints.
	ProfilingHandlers = map[string]http.Handler{
		"/debug/pprof":         http.HandlerFunc(redirectTo("/debug/pprof/")),
		"/debug/pprof/":        http.HandlerFunc(pprof.Index),
		"/debug/pprof/profile": http.HandlerFunc(pprof.Profile),
		"/debug/pprof/symbol":  http.HandlerFunc(pprof.Symbol),
		"/debug/pprof/trace":   http.HandlerFunc(pprof.Trace),
	}
)

// redirectTo redirects request to a certain destination.
func redirectTo(to string) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		http.Redirect(rw, req, to, http.StatusFound)
	}
}
