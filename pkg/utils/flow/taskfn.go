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

package flow

import (
	"context"
	"github.com/gardener/gardener/pkg/utils"
	"time"
)

// TaskFn is a payload function of a task.
type TaskFn func(ctx context.Context) error

// RecoverFn is a function that can recover an error.
type RecoverFn func(ctx context.Context, err error) error

// EmptyTaskFn is a TaskFn that does nothing (returns nil).
var EmptyTaskFn TaskFn = func(ctx context.Context) error { return nil }

// SimpleTaskFn converts the given function to a TaskFn, disrespecting any context.Context it is being given.
// deprecated: Only used during transition period. Do not use for new functions.
func SimpleTaskFn(f func() error) TaskFn {
	return func(ctx context.Context) error {
		return f()
	}
}

// SkipIf returns a TaskFn that does nothing if the condition is true, otherwise the function
// will be executed once called.
func (t TaskFn) SkipIf(condition bool) TaskFn {
	if condition {
		return EmptyTaskFn
	}
	return t
}

// DoIf returns a TaskFn that will be executed if the condition is true when it is called.
// Otherwise, it will do nothing when called.
func (t TaskFn) DoIf(condition bool) TaskFn {
	return t.SkipIf(!condition)
}

// Retry returns a TaskFn that is retried until the timeout is reached.
func (t TaskFn) Retry(interval time.Duration) TaskFn {
	return func(ctx context.Context) error {
		return utils.RetryUntil(ctx, interval, func() (ok, severe bool, err error) {
			if err := t(ctx); err != nil {
				return false, false, err
			}
			return true, false, nil
		})
	}
}

// RetryUntilTimeout returns a TaskFn that is retried until the timeout is reached.
func (t TaskFn) RetryUntilTimeout(interval, timeout time.Duration) TaskFn {
	return func(ctx context.Context) error {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()

		return utils.RetryUntil(ctx, interval, func() (ok, severe bool, err error) {
			if err := t(ctx); err != nil {
				return false, false, err
			}
			return true, false, nil
		})
	}
}

// ToRecoverFn converts the TaskFn to a RecoverFn that ignores the incoming error.
func (t TaskFn) ToRecoverFn() RecoverFn {
	return func(ctx context.Context, _ error) error {
		return t(ctx)
	}
}

// Recover creates a new TaskFn that recovers an error with the given RecoverFn.
func (t TaskFn) Recover(recoverFn RecoverFn) TaskFn {
	return func(ctx context.Context) error {
		if err := t(ctx); err != nil {
			return recoverFn(ctx, err)
		}
		return nil
	}
}

// RecoverTimeout creates a new TaskFn that recovers an error that satisfies `utils.IsTimedOut` with the given RecoverFn.
func (t TaskFn) RecoverTimeout(recoverFn RecoverFn) TaskFn {
	return t.Recover(func(ctx context.Context, err error) error {
		if utils.IsTimedOut(err) {
			return recoverFn(ctx, err)
		}
		return err
	})
}
