// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package common

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// DefaultWatchdogInterval is the default interval between checks performed by watchdogs.
	DefaultWatchdogInterval = 30 * time.Second
	// DefaultWatchdogTimeout is the default timeout for checks performed by watchdogs.
	DefaultWatchdogTimeout = 2 * time.Minute
	// DefaultWatchdogTTL is the default watchdog TTL.
	DefaultWatchdogTTL = 10 * time.Minute
)

// WatchdogManager manages Watchdog instances for multiple namespaces.
type WatchdogManager interface {
	// GetResultAndContext returns the result and context from the watchdog for the given namespace.
	// If the watchdog does not exist yet, it is created using the given context, client, namespace, and shoot name.
	// If the watchdog result is true, the given context is added to the watchdog mapped to the given key,
	// and a cleanup function for properly removing it is also returned.
	GetResultAndContext(ctx context.Context, c client.Client, namespace, shootName, key string) (bool, context.Context, func(), error)
}

// GetOwnerCheckResultAndContext returns the result and context from the watchdog for the given namespace managed by
// the default owner check WatchdogManager.
func GetOwnerCheckResultAndContext(ctx context.Context, c client.Client, namespace, shootName, key string) (bool, context.Context, func(), error) {
	return GetDefaultOwnerCheckWatchdogManager().GetResultAndContext(ctx, c, namespace, shootName, key)
}

// GetDefaultOwnerCheckWatchdogManager returns the default owner check WatchdogManager.
func GetDefaultOwnerCheckWatchdogManager() WatchdogManager {
	defaultOwnerCheckWatchdogManagerMutex.Lock()
	defer defaultOwnerCheckWatchdogManagerMutex.Unlock()

	if defaultOwnerCheckWatchdogManager == nil {
		defaultOwnerCheckWatchdogManager = NewWatchdogManager(
			NewCheckerWatchdogFactory(
				NewOwnerCheckerFactory(
					net.DefaultResolver,
					log.Log.WithName("owner-checker"),
				),
				DefaultWatchdogInterval,
				DefaultWatchdogTimeout,
				clock.RealClock{},
				log.Log.WithName("owner-check-watchdog"),
			),
			DefaultWatchdogTTL,
			clock.RealClock{},
			log.Log.WithName("owner-check-watchdog-manager"),
		)
	}
	return defaultOwnerCheckWatchdogManager
}

var (
	defaultOwnerCheckWatchdogManager      WatchdogManager
	defaultOwnerCheckWatchdogManagerMutex sync.Mutex
)

// NewWatchdogManager creates a new WatchdogManager using the given watchdog factory, ttl, and logger.
func NewWatchdogManager(watchdogFactory WatchdogFactory, ttl time.Duration, clk clock.Clock, logger logr.Logger) WatchdogManager {
	return &watchdogManager{
		watchdogFactory: watchdogFactory,
		ttl:             ttl,
		clock:           clk,
		logger:          logger,
		watchdogs:       make(map[string]Watchdog),
		timers:          make(map[string]clock.Timer),
	}
}

type watchdogManager struct {
	watchdogFactory WatchdogFactory
	ttl             time.Duration
	clock           clock.Clock
	logger          logr.Logger
	watchdogs       map[string]Watchdog
	watchdogsMutex  sync.Mutex
	timers          map[string]clock.Timer
	timersMutex     sync.Mutex
}

// GetResultAndContext returns the result and context from the watchdog for the given namespace.
// If the watchdog does not exist yet, it is created using the given context, client, namespace, and shoot name.
// If the watchdog result is true, the given context is added to the watchdog mapped to the given key,
// and a cleanup function for properly removing it is also returned.
func (m *watchdogManager) GetResultAndContext(ctx context.Context, c client.Client, namespace, shootName, key string) (bool, context.Context, func(), error) {
	watchdog, err := m.getWatchdog(ctx, c, namespace, shootName)
	if err != nil {
		return false, ctx, nil, err
	}
	if watchdog == nil {
		return true, ctx, nil, nil
	}
	result, err := watchdog.Result()
	if err != nil {
		return false, ctx, nil, err
	}
	if !result {
		return false, ctx, nil, nil
	}
	var firstAdded bool
	if ctx, firstAdded = watchdog.AddContext(ctx, key); firstAdded {
		m.cancelWatchdogRemoval(namespace)
	}
	cleanup := func() {
		if lastRemoved := watchdog.RemoveContext(key); lastRemoved {
			m.scheduleWatchdogRemoval(namespace)
		}
	}
	return true, ctx, cleanup, nil
}

func (m *watchdogManager) getWatchdog(ctx context.Context, c client.Client, namespace, shootName string) (Watchdog, error) {
	m.watchdogsMutex.Lock()
	defer m.watchdogsMutex.Unlock()

	watchdog, ok := m.watchdogs[namespace]
	if !ok {
		var err error
		watchdog, err = m.watchdogFactory.NewWatchdog(ctx, c, namespace, shootName)
		if err != nil {
			return nil, err
		}
		if watchdog == nil {
			return nil, nil
		}

		m.logger.Info("Starting watchdog", "namespace", namespace)
		watchdog.Start(context.Background())
		m.scheduleWatchdogRemoval(namespace)

		m.watchdogs[namespace] = watchdog
	}

	return watchdog, nil
}

func (m *watchdogManager) removeWatchdog(namespace string) {
	m.watchdogsMutex.Lock()
	defer m.watchdogsMutex.Unlock()

	if watchdog, ok := m.watchdogs[namespace]; ok {
		m.logger.Info("Stopping watchdog", "namespace", namespace)
		watchdog.Stop()

		delete(m.watchdogs, namespace)
	}
}

func (m *watchdogManager) scheduleWatchdogRemoval(namespace string) {
	m.timersMutex.Lock()
	defer m.timersMutex.Unlock()

	if timer, ok := m.timers[namespace]; !ok {
		timer = m.clock.AfterFunc(m.ttl, func() {
			m.removeWatchdog(namespace)
		})
		m.timers[namespace] = timer
	} else {
		timer.Reset(m.ttl)
	}
}

func (m *watchdogManager) cancelWatchdogRemoval(namespace string) {
	m.timersMutex.Lock()
	defer m.timersMutex.Unlock()

	if timer, ok := m.timers[namespace]; ok {
		timer.Stop()
		delete(m.timers, namespace)
	}
}
