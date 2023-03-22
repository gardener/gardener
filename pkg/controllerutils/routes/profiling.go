// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package routes

import (
	"net/http"
	"net/http/pprof"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	profilingHandlers = map[string]http.HandlerFunc{
		"/debug/pprof":         redirectTo("/debug/pprof/"),
		"/debug/pprof/":        pprof.Index,
		"/debug/pprof/profile": pprof.Profile,
		"/debug/pprof/symbol":  pprof.Symbol,
		"/debug/pprof/trace":   pprof.Trace,
	}
)

// Profiling adds handlers for pprof under /debug/pprof.
// This is similar to routes.Profiling from the API server library (uses the same paths).
// But instead of adding handlers to a mux.PathRecorderMux, it allows adding it to a manager.Manager.
type Profiling struct{}

// AddToManager adds the profiling handlers to the given Manager.
func (Profiling) AddToManager(mgr manager.Manager) error {
	for path, handler := range profilingHandlers {
		if err := mgr.AddMetricsExtraHandler(path, handler); err != nil {
			return err
		}
	}
	return nil
}

// redirectTo redirects request to a certain destination.
func redirectTo(to string) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		http.Redirect(rw, req, to, http.StatusFound)
	}
}
