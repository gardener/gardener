// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	mutex         sync.RWMutex
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
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.started {
		return
	}

	p.health = true
	p.timer = time.NewTimer(p.resetDuration)
	p.started = true
	p.stopCh = make(chan struct{})

	go func() {
		for {
			select {
			case <-p.timer.C:
				p.Set(false)
			case <-p.stopCh:
				p.timer.Stop()
				return
			}
		}
	}()
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

// Set sets the current health status. When the health status is 'true' then the timer is reset.
func (p *periodicHealthz) Set(health bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.health = health && p.started

	if health && p.started {
		p.timer.Reset(p.resetDuration)
	}
}
