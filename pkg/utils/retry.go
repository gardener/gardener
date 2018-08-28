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
)

// NeverStop is a channel that is always open. Can be used for never stopping a Retry computation.
var NeverStop = make(chan struct{})

// ConditionFunc is a function that reports whether it completed successfully or
// whether it had an error. Via the additional <severe> flag, it shows whether the
// error has been sever enough to cancel a retrying computation.
type ConditionFunc func() (ok, severe bool, err error)

// TimedOut is an error that occurs if an operation times out. Optionally yields the LastError,
// if any.
type TimedOut struct {
	// LastError is the last error that occurred before an operation timed out. May be nil.
	LastError error
	// WaitTime is the total time that was waited an operation to complete.
	WaitTime time.Duration
}

func (t *TimedOut) Error() string {
	if t.LastError == nil {
		return fmt.Sprintf("timed out after %s, no severe error occured", t.WaitTime)
	}
	return fmt.Sprintf("timed out after %s, last error: %v", t.WaitTime, t.LastError)
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
		} else if success {
			return nil
		}

		_, open := <-stopCh
		if !open {
			return &TimedOut{LastError: lastError, WaitTime: time.Since(startTime)}
		}

		time.Sleep(interval)
	}
}
