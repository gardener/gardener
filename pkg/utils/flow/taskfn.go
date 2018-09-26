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
	"github.com/gardener/gardener/pkg/utils"
	"time"
)

// TaskFn is a payload function of a task.
type TaskFn func() error

// RecoverFn is a function that can recover an error.
type RecoverFn func(error) error

// EmptyTaskFn is a TaskFn that does nothing (returns nil).
var EmptyTaskFn TaskFn = func() error { return nil }

// AlwaysNonSevere is an error predicate that always reports an error as non severe.
func AlwaysNonSevere(_ error) bool {
	return false
}

// ToConditionFunc converts a TaskFn to a wait.ConditionFunc. This is useful if
// retry utilities of the wait library should be used.
// The isSevereError function determines if an error is severe enough that the retrying
// shall immediately be canceled.
func (t TaskFn) ToConditionFunc(isSevereErr func(error) bool) utils.ConditionFunc {
	return func() (ok bool, severe bool, err error) {
		err = t()
		if err != nil {
			return false, isSevereErr(err), err
		}
		return true, false, nil
	}
}

// ToSimpleConditionFunc converts a TaskFn to a wait.ConditionFunc that always reports
// its errors as non severe such that a retry will always be done.
func (t TaskFn) ToSimpleConditionFunc() utils.ConditionFunc {
	return t.ToConditionFunc(AlwaysNonSevere)
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

// RetryUntilTimeout returns a TaskFn that is retried until the timeout is reached.
func (t TaskFn) RetryUntilTimeout(interval time.Duration, timeout time.Duration) TaskFn {
	return func() error {
		return utils.Retry(interval, timeout, t.ToSimpleConditionFunc())
	}
}

// ToRecoverFn converts the TaskFn to a RecoverFn that ignores the incoming error.
func (t TaskFn) ToRecoverFn() RecoverFn {
	return func(_ error) error {
		return t()
	}
}

// Recover creates a new TaskFn that recovers an error with the given RecoverFn.
func (t TaskFn) Recover(recoverFn RecoverFn) TaskFn {
	return func() error {
		if err := t(); err != nil {
			return recoverFn(err)
		}
		return nil
	}
}

// RecoverTimeout creates a new TaskFn that recovers a timeout error with the given RecoverFn.
func (t TaskFn) RecoverTimeout(recoverFn RecoverFn) TaskFn {
	return t.Recover(func(err error) error {
		if utils.IsTimedOut(err) {
			return recoverFn(err)
		}
		return err
	})
}
