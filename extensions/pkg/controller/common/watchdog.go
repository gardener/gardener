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
)

// Watchdog provides a method for starting a goroutine that regularly checks if a certain condition is true.
type Watchdog interface {
	// Start starts a goroutine that regularly checks if a certain condition is true.
	// If the check fails with an error or returns false, it cancels the returned context and returns.
	Start(ctx context.Context) (context.Context, context.CancelFunc)
}

// NewCheckerWatchdog creates a new Watchdog that checks if the condition checked by the given checker is true
// every given interval, using the given logger.
func NewCheckerWatchdog(checker Checker, interval time.Duration, logger logr.Logger) Watchdog {
	return &checkerWatchdog{
		checker:  checker,
		interval: interval,
		logger:   logger,
	}
}

type checkerWatchdog struct {
	checker  Checker
	interval time.Duration
	logger   logr.Logger
}

// Start starts a goroutine that checks if the condition checked by the watchdog checker is true every watchdog interval.
// If the check fails with an error or returns false, it cancels the returned context and returns.
func (w *checkerWatchdog) Start(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(w.interval):
			}

			if ok, err := w.checker.Check(ctx); err != nil || !ok {
				if err != nil {
					w.logger.Error(err, "Watchdog check failed with an error, cancelling context")
				} else {
					w.logger.Info("Watchdog check returned false, cancelling context")
				}
				cancel()
				return
			}
		}
	}()
	return ctx, cancel
}
