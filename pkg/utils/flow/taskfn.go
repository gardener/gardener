// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/gardener/gardener/pkg/utils/retry"
)

// ContextWithTimeout is context.WithTimeout. Exposed for testing.
var ContextWithTimeout = context.WithTimeout

// TaskFn is a payload function of a task.
type TaskFn func(ctx context.Context) error

// RecoverFn is a function that can recover an error.
type RecoverFn func(ctx context.Context, err error) error

// Timeout returns a TaskFn that is bound to a context which times out.
func (t TaskFn) Timeout(timeout time.Duration) TaskFn {
	return func(ctx context.Context) error {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()

		return t(ctx)
	}
}

type retryReporterContextKey struct{}

type retryReporterValue struct {
	id       TaskID
	reporter RetryReporter
}

func withRetryReporter(ctx context.Context, id TaskID, reporter RetryReporter) context.Context {
	return context.WithValue(ctx, retryReporterContextKey{}, &retryReporterValue{id: id, reporter: reporter})
}

// ReportRetry reports that the current task failed with err and will be retried.
// It extracts the reporter from the context (injected by the flow engine) and calls it.
// This is a no-op if no reporter is present in the context.
//
// Call this inside a custom retry loop to surface the last error to the progress reporter.
// When using TaskFn.RetryUntilTimeout, ReportRetry is called automatically on each failure.
func ReportRetry(ctx context.Context, err error) {
	v, ok := ctx.Value(retryReporterContextKey{}).(*retryReporterValue)
	if !ok || v == nil {
		return
	}
	v.reporter.ReportRetry(ctx, v.id, err)
}

// RetryUntilTimeout returns a TaskFn that is retried until the timeout is reached.
// On each failed attempt, ReportRetry is called automatically so that progress reporters
// can surface the last error without any additional wiring.
func (t TaskFn) RetryUntilTimeout(interval, timeout time.Duration) TaskFn {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		return retry.Until(ctx, interval, func(ctx context.Context) (done bool, err error) {
			if err := t(ctx); err != nil {
				ReportRetry(ctx, err)
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

// ParallelN returns a function that runs the given TaskFns in parallel by spawning N workers,
// collecting their errors in a multierror. If N <= 0, then N will be defaulted to len(fns).
func ParallelN(n int, fns ...TaskFn) TaskFn {
	workers := n
	if n <= 0 {
		workers = len(fns)
	}
	return func(ctx context.Context) error {
		var (
			wg     sync.WaitGroup
			fnsCh  = make(chan TaskFn)
			errCh  = make(chan error)
			result error
		)

		for i := 0; i < workers; i++ {
			wg.Go(func() {
				for fn := range fnsCh {
					errCh <- fn(ctx)
				}
			})
		}

		go func() {
			for _, f := range fns {
				fnsCh <- f
			}
			close(fnsCh)
		}()

		go func() {
			defer close(errCh)
			wg.Wait()
		}()

		for err := range errCh {
			if err != nil {
				result = multierror.Append(result, err)
			}
		}
		return result
	}
}

// Parallel runs the given TaskFns in parallel, collecting their errors in a multierror.
func Parallel(fns ...TaskFn) TaskFn {
	return ParallelN(len(fns), fns...)
}

// ParallelExitOnError runs the given TaskFns in parallel and stops execution as soon as one TaskFn returns an error.
func ParallelExitOnError(fns ...TaskFn) TaskFn {
	return func(ctx context.Context) error {
		var (
			wg sync.WaitGroup
			// make sure all other goroutines can send their result if one task fails to not block and leak them
			errors         = make(chan error, len(fns))
			subCtx, cancel = context.WithCancel(ctx)
		)

		// cancel any remaining parallel tasks on error,
		// though we will not wait until all tasks have finished
		defer cancel()

		for _, fn := range fns {
			t := fn

			wg.Go(func() {
				errors <- t(subCtx)
			})
		}

		go func() {
			// close errors channel as soon as all tasks finished to stop range operator in for loop reading from channel
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
