// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package retry

import (
	"context"
	"fmt"
	"time"

	contextutils "github.com/gardener/gardener/pkg/utils/context"
)

type lastErrorAggregator struct {
	lastError error
}

// Minor implements ErrorAggregator.
func (l *lastErrorAggregator) Minor(err error) {
	l.lastError = err
}

// Severe implements ErrorAggregator.
func (l *lastErrorAggregator) Severe(err error) {
	l.lastError = err
}

// Error implements ErrorAggregator.
func (l *lastErrorAggregator) Error() error {
	return l.lastError
}

// NewLastErrorAggregator returns an ErrorAggregator that only keeps the last error it recorded.
func NewLastErrorAggregator() ErrorAggregator {
	return &lastErrorAggregator{}
}

// New implements ErrorAggregatorFactory.
func (f ErrorAggregatorFactoryFunc) New() ErrorAggregator {
	return f()
}

// DefaultErrorAggregatorFactory returns the default ErrorAggregatorFactory.
func DefaultErrorAggregatorFactory() ErrorAggregatorFactory {
	return ErrorAggregatorFactoryFunc(NewLastErrorAggregator)
}

// New implements IntervalFactory.
func (f IntervalFactoryFunc) New(interval time.Duration) WaitFunc {
	return f(interval)
}

// NewIntervalFactory returns a new IntervalFactory using the given contextutils.Ops.
func NewIntervalFactory(contextOps contextutils.Ops) IntervalFactory {
	return IntervalFactoryFunc(func(interval time.Duration) WaitFunc {
		return func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextOps.WithTimeout(ctx, interval)
		}
	})
}

var defaultIntervalFactory = NewIntervalFactory(contextutils.DefaultOps())

// DefaultIntervalFactory returns the default IntervalFactory.
func DefaultIntervalFactory() IntervalFactory {
	return defaultIntervalFactory
}

// SevereError indicates an operation was not successful due to the given error and cannot be retried.
func SevereError(severeErr error) (bool, error) {
	return true, severeErr
}

// MinorError indicates an operation was not successful due to the given error but can be retried.
func MinorError(minorErr error) (bool, error) {
	return false, minorErr
}

// Ok indicates that an operation was successful and does not need to be retried.
func Ok() (bool, error) {
	return true, nil
}

// NotOk indicates that an operation was not successful but can be retried.
// It does not indicate an error. For better error reporting, consider MinorError.
func NotOk() (bool, error) {
	return false, nil
}

// MinorOrSevereError returns a "severe" error in case the retry count exceeds the threshold. Otherwise, it returns
// a "minor" error.
func MinorOrSevereError(retryCountUntilSevere, threshold int, err error) (bool, error) {
	if retryCountUntilSevere > threshold {
		return SevereError(err)
	}
	return MinorError(err)
}

// Error is an error that occurred during a retry operation.
type Error struct {
	ctxError error
	err      error
}

// Unwrap implements the Unwrap function
// https://golang.org/pkg/errors/#Unwrap
func (e *Error) Unwrap() error {
	if e.err != nil {
		return e.err
	}
	return e.ctxError
}

// Error implements error.
func (e *Error) Error() string {
	if e.err != nil {
		return fmt.Sprintf("retry failed with %v, last error: %v", e.ctxError, e.err)
	}
	return fmt.Sprintf("retry failed with %v", e.ctxError)
}

// NewError returns a new error with the given context error and error. The non-context error is optional.
func NewError(ctxError, err error) error {
	return &Error{ctxError, err}
}

// UntilFor keeps retrying the given Func until it either errors severely or the context expires.
// Between each try, it waits using the context of the given WaitFunc.
func UntilFor(ctx context.Context, waitFunc WaitFunc, agg ErrorAggregator, f Func) error {
	for {
		done, err := f(ctx)
		if err != nil {
			if done {
				agg.Severe(err)
				return agg.Error()
			}

			agg.Minor(err)
		} else if done {
			return nil
		}

		if err := func() error {
			wait, cancel := waitFunc(ctx)
			defer cancel()

			waitDone := wait.Done()
			ctxDone := ctx.Done()

			select {
			case <-waitDone:
				select {
				case <-ctxDone:
					return NewError(ctx.Err(), agg.Error())
				default:
					return nil
				}
			case <-ctxDone:
				return NewError(ctx.Err(), agg.Error())
			}
		}(); err != nil {
			return err
		}
	}
}

type ops struct {
	intervalFactory        IntervalFactory
	errorAggregatorFactory ErrorAggregatorFactory
	contextOps             contextutils.Ops
}

// Until implements Ops.
func (o *ops) Until(ctx context.Context, interval time.Duration, f Func) error {
	return UntilFor(ctx, o.intervalFactory.New(interval), o.errorAggregatorFactory.New(), f)
}

// UntilTimeout implements Ops.
func (o *ops) UntilTimeout(ctx context.Context, interval, timeout time.Duration, f Func) error {
	ctx, cancel := o.contextOps.WithTimeout(ctx, timeout)
	defer cancel()
	return o.Until(ctx, interval, f)
}

// NewOps returns the new ops with the given IntervalFactory, ErrorAggregatorFactory and contextutils.Ops.
func NewOps(intervalFactory IntervalFactory, errorAggregatorFactory ErrorAggregatorFactory, contextOps contextutils.Ops) Ops {
	return &ops{intervalFactory, errorAggregatorFactory, contextOps}
}

var defaultOps = NewOps(DefaultIntervalFactory(), DefaultErrorAggregatorFactory(), contextutils.DefaultOps())

// DefaultOps returns the default Ops with the DefaultIntervalFactory, DefaultErrorAggregatorFactory and contextutils.DefaultOps.
func DefaultOps() Ops {
	return defaultOps
}
