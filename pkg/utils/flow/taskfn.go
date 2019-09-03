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
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/hashicorp/go-multierror"
)

var (
	// ContextWithTimeout is context.WithTimeout. Exposed for testing.
	ContextWithTimeout = context.WithTimeout
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
// Deprecated: Retry handling should be done in the function itself, if necessary.
func (t TaskFn) Retry(interval time.Duration) TaskFn {
	return func(ctx context.Context) error {
		return retry.Until(ctx, interval, func(ctx context.Context) (done bool, err error) {
			if err := t(ctx); err != nil {
				return retry.MinorError(err)
			}
			return retry.Ok()
		})
	}
}

// Timeout returns a TaskFn that is bound to a context which times out.
func (t TaskFn) Timeout(timeout time.Duration) TaskFn {
	return func(ctx context.Context) error {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()

		return t(ctx)
	}
}

// RetryUntilTimeout returns a TaskFn that is retried until the timeout is reached.
func (t TaskFn) RetryUntilTimeout(interval, timeout time.Duration) TaskFn {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		return retry.Until(ctx, interval, func(ctx context.Context) (done bool, err error) {
			if err := t(ctx); err != nil {
				return retry.MinorError(err)
			}
			return retry.Ok()
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
			if ctx.Err() != nil {
				return err
			}
			return recoverFn(ctx, err)
		}
		return nil
	}
}

// Sequential runs the given TaskFns sequentially.
func Sequential(fns ...TaskFn) TaskFn {
	return func(ctx context.Context) error {
		for _, fn := range fns {
			if err := fn(ctx); err != nil {
				return err
			}

			if err := ctx.Err(); err != nil {
				return err
			}
		}
		return nil
	}
}

// ParallelWithSubmitter runs the given TaskFns in parallel with the given Submitter, collecting their errors in a multierror.
func ParallelWithSubmitter(s Submitter, fns ...TaskFn) TaskFn {
	return func(ctx context.Context) error {
		var (
			wg     sync.WaitGroup
			errors = make(chan error)
			result error
		)

		for _, fn := range fns {
			t := fn
			wg.Add(1)
			s.Submit(func() {
				defer wg.Done()
				errors <- t(ctx)
			})
		}

		go func() {
			defer close(errors)
			wg.Wait()
		}()

		for err := range errors {
			if err != nil {
				result = multierror.Append(result, err)
			}
		}
		return result
	}
}

// Parallel runs the given TaskFns in parallel, collecting their errors in a multierror.
func Parallel(fns ...TaskFn) TaskFn {
	return ParallelWithSubmitter(UnlimitedSubmitter, fns...)
}

// ParallelExitOnError runs the given TaskFns in parallel and stops execution as soon as one TaskFn returns an error.
func ParallelExitOnError(fns ...TaskFn) TaskFn {
	return func(ctx context.Context) error {
		var (
			wg             sync.WaitGroup
			errors         = make(chan error)
			subCtx, cancel = context.WithCancel(ctx)
		)
		defer cancel()

		for _, fn := range fns {
			t := fn
			wg.Add(1)
			go func() {
				defer wg.Done()
				errors <- t(subCtx)
			}()
		}

		go func() {
			defer close(errors)
			wg.Wait()
		}()

		for err := range errors {
			if err != nil {
				return err
			}
		}
		return nil
	}
}

// Submitter is an interface to run functions in parallel.
type Submitter interface {
	// Submit runs the given function asynchronously. This function should not block
	// during the execution of f.
	Submit(f func())
}

func workQueue() (chan<- func(), <-chan func()) {
	var (
		out   = make(chan func())
		in    = make(chan func())
		queue []func()
		exit  bool
	)

	go func() {
		defer close(out)
		for len(queue) != 0 || !exit {
			if len(queue) == 0 {
				f, ok := <-in
				if !ok {
					exit = true
					continue
				}

				queue = append(queue, f)
				continue
			}

			select {
			case f, ok := <-in:
				if !ok {
					exit = true
					continue
				}

				queue = append(queue, f)
			case out <- queue[0]:
				queue = queue[1:]
			}
		}
	}()

	return in, out
}

// LimitSubmitter contains information about the pool size which is used
// to limit the submission of functions in parallel.
type LimitSubmitter struct {
	submitter Submitter
	in        chan<- func()
	size      int
	running   bool
}

func (l *LimitSubmitter) run(work <-chan func()) {
	var (
		exit bool
		done = make(chan struct{})
		ct   int
	)
	defer close(done)

	for ct > 0 || !exit {
		if ct < l.size {
			w, ok := <-work
			if !ok {
				exit = true
				continue
			}

			if !exit {
				ct++
				l.submitter.Submit(func() {
					w()
					done <- struct{}{}
				})
			}
			continue
		}

		<-done
		ct--
	}
}

// Start starts the workers of the LimitSubmitter.
func (l *LimitSubmitter) Start() {
	if !l.running {
		l.running = true
		in, out := workQueue()
		go l.run(out)
		l.in = in
	}
}

// Stop stops the workers of the LimitSubmitter.
func (l *LimitSubmitter) Stop() {
	if l.running {
		l.running = false
		close(l.in)
		l.in = nil
	}
}

// Submit dispatches the given function to the LimitSubmitter.
// The LimitSubmitter must be started before before calling this function.
func (l *LimitSubmitter) Submit(f func()) {
	if !l.running {
		panic("cannot submit on non-running LimitSubmitter")
	}
	l.in <- f
}

// NewLimitSubmitter returns a new instance of a LimitSubmitter and a submit pool that has the given size.
func NewLimitSubmitter(submitter Submitter, size int) *LimitSubmitter {
	s := &LimitSubmitter{
		submitter: submitter,
		size:      size,
	}
	return s
}

type unlimitedSubmitter struct{}

// Submit implements Submitter.Submit
func (unlimitedSubmitter) Submit(f func()) {
	go f()
}

// UnlimitedSubmitter is a submitter with an unlimited pool to submit functions.
var UnlimitedSubmitter = unlimitedSubmitter{}
