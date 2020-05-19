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
	"sync"
	"time"
)

// PeriodicHealthManagerName is the name of the periodic health manager.
const PeriodicHealthManagerName = "periodic"

// NewPeriodicHealthz returns a health manager that automatically sets the health status to false after the given reset
// duration. The timer is reset again when the health status is true (i.e., a running timer is reset and starts again
// from the beginning).
func NewPeriodicHealthz(resetDuration time.Duration) Manager {
	return &periodicHealthz{resetDuration: resetDuration}
}

type periodicHealthz struct {
	mutex         sync.Mutex
	health        bool
	timer         *time.Timer
	resetDuration time.Duration
	started       bool
	stopCh        chan struct{}
}

// Name returns the name of the health manager.
func (p *periodicHealthz) Name() string {
	return PeriodicHealthManagerName
}

// Start starts the health manager.
func (p *periodicHealthz) Start() {
	if p.started {
		return
	}

	p.Set(true)
	p.timer = time.NewTimer(p.resetDuration)
	p.started = true
	p.stopCh = make(chan struct{})

	go func() {
		for {
			select {
			case <-p.timer.C:
				p.Set(false)
				p.timer.Reset(p.resetDuration)
			case <-p.stopCh:
				p.timer.Stop()
			}
		}
	}()
}

// Stop starts the health manager.
func (p *periodicHealthz) Stop() {
	p.Set(false)
	close(p.stopCh)
	p.started = false
}

// Get returns the current health status.
func (p *periodicHealthz) Get() bool {
	return p.health
}

// Set sets the current health status. When the health status is 'true' then the timer is reset.
func (p *periodicHealthz) Set(health bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.health = health

	if health && p.started {
		p.timer.Reset(p.resetDuration)
	}
}
