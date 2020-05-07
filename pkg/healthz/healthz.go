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

package healthz

import (
	"net/http"
)

// Manager is an interface for health managers.
type Manager interface {
	// Name returns the name of the health manager.
	Name() string
	// Start starts the health manager.
	Start()
	// Stop stops the health manager.
	Stop()
	// Get returns the current health status.
	Get() bool
	// Set updates the current health status with the given value.
	Set(bool)
}

// HandlerFunc returns a HTTP handler that responds with 200 OK status code if the given health manager returns true,
// otherwise 500 Internal Server Error status code will be returned.
func HandlerFunc(h Manager) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.Get() {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}
