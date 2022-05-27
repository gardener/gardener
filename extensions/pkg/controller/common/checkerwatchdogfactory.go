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
	"time"

	"github.com/go-logr/logr"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WatchdogFactory creates Watchdog instances.
type WatchdogFactory interface {
	// NewWatchdog creates a new Watchdog using the given context, client, namespace, and shoot name.
	NewWatchdog(ctx context.Context, c client.Client, namespace, shootName string) (Watchdog, error)
}

// NewCheckerWatchdogFactory creates a new WatchdogFactory that uses NewCheckerWatchdog to create Watchdog instances.
func NewCheckerWatchdogFactory(checkerFactory CheckerFactory, interval, timeout time.Duration, clock clock.Clock, logger logr.Logger) WatchdogFactory {
	return &checkerWatchdogFactory{
		checkerFactory: checkerFactory,
		interval:       interval,
		timeout:        timeout,
		clock:          clock,
		logger:         logger,
	}
}

type checkerWatchdogFactory struct {
	checkerFactory CheckerFactory
	interval       time.Duration
	timeout        time.Duration
	clock          clock.Clock
	logger         logr.Logger
}

// NewWatchdog creates a new Watchdog using the given context, client, namespace, and shoot name.
// It uses the checker factory to create a new Checker that is then passed to NewCheckerWatchdog.
// If the checker factory returns a nil Checker, this method returns a nil Watchdog.
func (f *checkerWatchdogFactory) NewWatchdog(ctx context.Context, c client.Client, namespace, shootName string) (Watchdog, error) {
	checker, err := f.checkerFactory.NewChecker(ctx, c, namespace, shootName)
	if err != nil {
		return nil, err
	}
	if checker == nil {
		return nil, nil
	}

	return NewCheckerWatchdog(checker, f.interval, f.timeout, f.clock, f.logger.WithName(namespace)), nil
}
