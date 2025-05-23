// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthz

import (
	"context"
	"errors"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

// Manager is an interface for health managers.
type Manager interface {
	// Name returns the name of the health manager.
	Name() string
	// Start starts the health manager.
	Start(context.Context) error
	// Stop stops the health manager.
	Stop()
	// Get returns the current health status.
	Get() bool
	// Set updates the current health status with the given value.
	Set(bool)
}

// CheckerFunc returns a new healthz.Checker that will pass only if the given health manager returns true.
func CheckerFunc(h Manager) healthz.Checker {
	return func(_ *http.Request) error {
		if !h.Get() {
			return errors.New("current health status is 'unhealthy'")
		}
		return nil
	}
}
