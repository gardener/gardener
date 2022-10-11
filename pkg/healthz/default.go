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
