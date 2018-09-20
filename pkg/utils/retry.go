// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils

import (
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/logger"
)

// NeverStop is a channel that is always open. Can be used for never stopping a Retry computation.
var NeverStop = make(chan struct{})

// ConditionFunc is a function that reports whether it completed successfully or
// whether it had an error. Via the additional <severe> flag, it shows whether the
// error has been sever enough to cancel a retrying computation.
type ConditionFunc func() (ok, severe bool, err error)

// NewTimedOut creates a new error that indicates a timeout after the given waitTime.
func NewTimedOut(waitTime time.Duration) error {
	return &timedOut{waitTime}
}

// NewTimedOutWithError creates a new error that indicates a timeout after the given waitTime caused
// by the given lastError.
func NewTimedOutWithError(waitTime time.Duration, lastError error) error {
	return &timedOutWithError{lastError, waitTime}
}

type timedOut struct {
	waitTime time.Duration
}

type timedOutWithError struct {
	lastError error
	waitTime  time.Duration
}

func (t *timedOutWithError) Cause() error {
	return t.lastError
}

func (t *timedOut) Error() string {
	return fmt.Sprintf("timed out after %s", t.waitTime)
}

func (t *timedOutWithError) Error() string {
	return fmt.Sprintf("timed out after %s, last error: %v", t.waitTime, t.lastError)
}

func newStopCh(timeout time.Duration, done <-chan struct{}) <-chan struct{} {
	stopCh := make(chan struct{})
	t := time.NewTimer(timeout)

	go func() {
		defer t.Stop()
		defer close(stopCh)

		select {
		case <-t.C:
		case <-done:
			return
		}
	}()
	return stopCh
}

// Retry retries <f> until it either succeeds, fails severely or times out.
// Between each runs, it sleeps for <interval>.
func Retry(interval time.Duration, timeout time.Duration, f ConditionFunc) error {
	done := make(chan struct{})
	defer close(done)

	return RetryUntil(interval, newStopCh(timeout, done), f)
}

// RetryUntil retries <f> until it either succeeds, fails severely or the given channel <stopCh>
// is closed. Between each tries, it sleeps for <interval>.
func RetryUntil(interval time.Duration, stopCh <-chan struct{}, f ConditionFunc) error {
	var (
		lastError error
		startTime = time.Now()
	)
	for {
		success, severe, err := f()
		if err != nil {
			if severe {
				return err
			}

			lastError = err
			logger.Logger.Error(err)
		} else if success {
			return nil
		}

		select {
		case <-stopCh:
			waitTime := time.Since(startTime)
			if lastError == nil {
				return NewTimedOut(waitTime)
			}
			return NewTimedOutWithError(waitTime, lastError)
		default:
		}

		time.Sleep(interval)
	}
}
