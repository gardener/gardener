// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
