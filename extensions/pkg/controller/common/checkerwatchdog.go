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
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/utils/clock"
)

// Watchdog manages a goroutine that regularly checks if a certain condition is true,
// and if the check fails or returns false, cancels multiple previously added contexts.
type Watchdog interface {
	// Start starts a goroutine that regularly checks if a certain condition is true.
	// If the check fails or returns false, it cancels all contexts.
	Start(ctx context.Context)
	// Stop stops the goroutine started by Start.
	Stop()
	// AddContext adds the given context to this watchdog mapped to the given key,
	// and returns a new context that will be cancelled if the condition check fails or returns false.
	// It returns true if the first context has been added, false otherwise.
	AddContext(ctx context.Context, key string) (context.Context, bool)
	// RemoveContext removes the context mapped to the given key from this watchdog.
	// It returns true if the last context has been removed, false otherwise.
	RemoveContext(key string) bool
	// Result returns the result of the last condition check.
	Result() (bool, error)
}

// NewCheckerWatchdog creates a new Watchdog that checks if the condition checked by the given checker is true
// every given interval, using the given clock and logger.
func NewCheckerWatchdog(checker Checker, interval, timeout time.Duration, clock clock.Clock, logger logr.Logger) Watchdog {
	return &checkerWatchdog{
		checker:         checker,
		interval:        interval,
		timeout:         timeout,
		clock:           clock,
		logger:          logger,
		ctxCancelFuncs:  make(map[string]context.CancelFunc),
		resultChan:      make(chan struct{}),
		resultReadyChan: make(chan struct{}),
		timerChan:       make(chan bool),
	}
}

type checkerWatchdog struct {
	checker             Checker
	interval            time.Duration
	timeout             time.Duration
	clock               clock.Clock
	logger              logr.Logger
	cancelFunc          context.CancelFunc
	ctxCancelFuncs      map[string]context.CancelFunc
	ctxCancelFuncsMutex sync.Mutex
	result              bool
	err                 error
	resultTime          time.Time
	resultMutex         sync.RWMutex
	resultChan          chan struct{}
	resultReadyChan     chan struct{}
	timerChan           chan bool
}

// Start starts a goroutine that checks if the condition checked by the watchdog checker is true every watchdog interval.
// If the check fails or returns false, it cancels all contexts.
func (w *checkerWatchdog) Start(ctx context.Context) {
	w.logger.V(1).Info("Starting watchdog")
	ctx, w.cancelFunc = context.WithCancel(ctx)
	timer := w.clock.NewTimer(w.interval)
	timer.Stop()
	go func() {
		for {
			// Wait for a timer event or a result request
			resultRequested := false
			select {
			case <-ctx.Done():
				return
			case reset := <-w.timerChan:
				// Stop the timer, then reset it if a reset was requested
				// Timers are not concurrency safe, therefore all timer updates are performed here
				timer.Stop()
				if reset {
					timer.Reset(w.interval)
				}
				continue
			case <-w.resultChan:
				resultRequested = true
				// If the last result is not older than w.interval, use it
				if !w.clock.Now().After(w.resultTime.Add(w.interval)) {
					w.resultReadyChan <- struct{}{}
					continue
				}
			case <-timer.C():
				timer.Reset(w.interval)
			}

			// Call checker and update result
			timeoutCtx, cancel := context.WithTimeout(ctx, w.timeout)
			result, err := w.checker.Check(timeoutCtx)
			cancel()
			w.setResult(result, err)

			// If a result was requested, notify the requester that the new result is available
			if resultRequested {
				w.resultReadyChan <- struct{}{}
			}

			// If the check failed or returned false, cancel all contexts
			if err != nil || !result {
				w.logger.Info("Cancelling all contexts")
				w.ctxCancelFuncsMutex.Lock()
				for _, cancelFunc := range w.ctxCancelFuncs {
					cancelFunc()
				}
				w.ctxCancelFuncsMutex.Unlock()
			}
		}
	}()
}

// Stop stops the goroutine started by Start.
func (w *checkerWatchdog) Stop() {
	w.logger.V(1).Info("Stopping watchdog")
	w.cancelFunc()
}

// AddContext adds the given context to this watchdog mapped to the given key,
// and returns a new context that will be cancelled if the condition check fails or returns false.
// It returns true if the first context has been added, false otherwise.
func (w *checkerWatchdog) AddContext(ctx context.Context, key string) (context.Context, bool) {
	w.logger.V(1).Info("Adding context", "key", key)

	w.ctxCancelFuncsMutex.Lock()
	ctx, cancelFunc := context.WithCancel(ctx)
	w.ctxCancelFuncs[key] = cancelFunc
	firstAdded := len(w.ctxCancelFuncs) == 1
	w.ctxCancelFuncsMutex.Unlock()

	// Reset the timer if the first context has been added
	if firstAdded {
		w.timerChan <- true
	}

	return ctx, firstAdded
}

// RemoveContext removes the context mapped to the given key from this watchdog.
// It returns true if the last context has been removed, false otherwise.
func (w *checkerWatchdog) RemoveContext(key string) bool {
	w.logger.V(1).Info("Removing context", "key", key)

	w.ctxCancelFuncsMutex.Lock()
	delete(w.ctxCancelFuncs, key)
	lastRemoved := len(w.ctxCancelFuncs) == 0
	w.ctxCancelFuncsMutex.Unlock()

	// Stop the timer if the last context has been removed
	if lastRemoved {
		w.timerChan <- false
	}

	return lastRemoved
}

// Result returns the result of the last condition check.
func (w *checkerWatchdog) Result() (bool, error) {
	// Request the result and wait for it to be updated if needed
	w.resultChan <- struct{}{}
	<-w.resultReadyChan

	w.resultMutex.RLock()
	defer w.resultMutex.RUnlock()
	return w.result, w.err
}

func (w *checkerWatchdog) setResult(result bool, err error) {
	w.resultMutex.Lock()
	defer w.resultMutex.Unlock()
	w.result, w.err, w.resultTime = result, err, w.clock.Now()
}
