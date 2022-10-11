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
	"time"

	"k8s.io/utils/clock"
)

// PeriodicHealthManagerName is the name of the periodic health manager.
const PeriodicHealthManagerName = "periodic"

// NewPeriodicHealthz returns a health manager that automatically sets the health status to false after the given reset
// duration. The timer is reset again when the health status is true (i.e., a running timer is reset and starts again
// from the beginning).
func NewPeriodicHealthz(clock clock.Clock, resetDuration time.Duration) Manager {
	return &periodicHealthz{clock: clock, resetDuration: resetDuration}
}

type periodicHealthz struct {
	clock         clock.Clock
	mutex         sync.RWMutex
	health        bool
	timer         clock.Timer
	resetDuration time.Duration
	started       bool
	stopCh        chan struct{}
}

// Name returns the name of the health manager.
func (p *periodicHealthz) Name() string {
	return PeriodicHealthManagerName
}

// Start starts the health manager.
func (p *periodicHealthz) Start(_ context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.started {
		return nil
	}

	p.health = true
	p.timer = p.clock.NewTimer(p.resetDuration)
	p.started = true
	p.stopCh = make(chan struct{})

	go func() {
		for {
			select {
			case <-p.timer.C():
				p.Set(false)
			case <-p.stopCh:
				p.timer.Stop()
				return
			}
		}
	}()

	return nil
}

// Stop stops the health manager.
func (p *periodicHealthz) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.health = false
	if !p.started {
		return
	}

	close(p.stopCh)
	p.started = false
}

// Get returns the current health status.
func (p *periodicHealthz) Get() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.health
}

// Set sets the current health status. When the health status is 'true' and the manager is started then the timer is
// reset.
func (p *periodicHealthz) Set(health bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.health = health

	if health && p.started {
		p.timer.Reset(p.resetDuration)
	}
}
