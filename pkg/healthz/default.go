// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthz

import (
	"context"
	"sync"
)

// DefaultHealthManagerName is the name of the default health manager.
const DefaultHealthManagerName = "default"

// NewDefaultHealthz returns a default health manager that stores the given health status and returns it.
func NewDefaultHealthz() Manager {
	return &defaultHealthz{}
}

type defaultHealthz struct {
	mutex   sync.RWMutex
	health  bool
	started bool
}

// Name returns the name of the health manager.
func (d *defaultHealthz) Name() string {
	return DefaultHealthManagerName
}

// Start starts the health manager.
func (d *defaultHealthz) Start(_ context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.started = true
	d.health = true
	return nil
}

// Stop stops the health manager.
func (d *defaultHealthz) Stop() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.started = false
	d.health = false
}

// Get returns the current health status.
func (d *defaultHealthz) Get() bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.health
}

// Set sets the current health status.
func (d *defaultHealthz) Set(health bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.health = health
}
