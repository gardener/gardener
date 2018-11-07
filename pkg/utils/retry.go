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
	"context"
	"fmt"
	"time"
)

// ConditionFunc is a function that reports whether it completed successfully or
// whether it had an error. Via the additional <severe> flag, it shows whether the
// error has been sever enough to cancel a retrying computation.
type ConditionFunc func() (ok, severe bool, err error)

func newTimedOut(waitTime time.Duration, lastError error) error {
	if lastError != nil {
		return &timedOutWithError{lastError, waitTime}
	}
	return &timedOut{waitTime}
}

// IsTimedOut determines whether the given error is a timed out error.
func IsTimedOut(err error) bool {
	switch err.(type) {
	case *timedOut, *timedOutWithError:
		return true
	default:
		return false
	}
}

// WaitTimeOfTimedOut returns the wait time of the given error if it implements the
// waitTimer interface:
// ```
// type waitTimer interface {
// 	WaitTime() time.Duration
// }
// If the given error does not implement the waitTimer interface, it just returns 0.
func WaitTimeOfTimedOut(err error) time.Duration {
	type waitTimer interface {
		WaitTime() time.Duration
	}
	if w, ok := err.(waitTimer); ok {
		return w.WaitTime()
	}
	return 0
}

// LastErrorOfTimedOutWithError returns the last error if the given error was a <timedOutWithError>.
// Otherwise, it just returns nil.
func LastErrorOfTimedOutWithError(err error) error {
	if t, ok := err.(*timedOutWithError); ok {
		return t.lastError
	}
	return nil
}

type timedOut struct {
	waitTime time.Duration
}

func (t *timedOut) WaitTime() time.Duration {
	return t.waitTime
}

type timedOutWithError struct {
	lastError error
	waitTime  time.Duration
}

func (t *timedOutWithError) WaitTime() time.Duration {
	return t.waitTime
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

// Retry retries <f> until it either succeeds, fails severely or times out.
// Between each runs, it sleeps for <interval>.
func Retry(interval time.Duration, timeout time.Duration, f ConditionFunc) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return RetryUntil(ctx, interval, f)
}

// RetryUntil retries <f> until it either succeeds, fails severely or the given channel <stopCh>
// is closed. Between each tries, it sleeps for <interval>. The function f is guaranteed to
// be executed at least once. During each execution, f can't be prematurely killed, thus an operation
// may run considerably longer than anticipated after closing the <stopCh>.
func RetryUntil(ctx context.Context, interval time.Duration, f ConditionFunc) error {
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

		if ctx.Err() != nil {
			waitTime := time.Since(startTime)
			return newTimedOut(waitTime, lastError)
		}

		time.Sleep(interval)

		if ctx.Err() != nil {
			waitTime := time.Since(startTime)
			return newTimedOut(waitTime, lastError)
		}
	}
}
