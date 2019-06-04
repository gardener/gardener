// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package retry

import (
	"context"
	"fmt"
	"time"

	utilcontext "github.com/gardener/gardener/pkg/utils/context"
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

// NewIntervalFactory returns a new IntervalFactory using the given utilcontext.Ops.
func NewIntervalFactory(contextOps utilcontext.Ops) IntervalFactory {
	return IntervalFactoryFunc(func(interval time.Duration) WaitFunc {
		return func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextOps.WithTimeout(ctx, interval)
		}
	})
}

var defaultIntervalFactory = NewIntervalFactory(utilcontext.DefaultOps())

// DefaultIntervalFactory returns the default IntervalFactory.
func DefaultIntervalFactory() IntervalFactory {
	return defaultIntervalFactory
}

// SevereError indicates an operation was not successful due to the given error and cannot be retried.
func SevereError(severeErr error) (done bool, err error) {
	return true, severeErr
}

// MinorError indicates an operation was not successful due to the given error but can be retried.
func MinorError(minorErr error) (done bool, err error) {
	return false, minorErr
}

// Ok indicates that an operation was successful and does not need to be retried.
func Ok() (done bool, err error) {
	return true, nil
}

// NotOk indicates that an operation was not successful but can be retried.
// It does not indicate an error. For better error reporting, consider MinorError.
func NotOk() (done bool, err error) {
	return false, nil
}

type retryError struct {
	ctxError error
	err      error
}

// Cause implements Causer.
func (r *retryError) Cause() error {
	if r.err != nil {
		return r.err
	}
	return r.ctxError
}

// Error implements error.
func (r *retryError) Error() string {
	if r.err != nil {
		return fmt.Sprintf("retry failed with %v, last error: %v", r.ctxError, r.err)
	}
	return fmt.Sprintf("retry failed with %v", r.ctxError)
}

// NewRetryError returns a new error with the given context error and error. The non-context error is optional.
func NewRetryError(ctxError, err error) error {
	return &retryError{ctxError, err}
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
					return NewRetryError(ctx.Err(), agg.Error())
				default:
					return nil
				}
			case <-ctxDone:
				return NewRetryError(ctx.Err(), agg.Error())
			}
		}(); err != nil {
			return err
		}
	}
}

type ops struct {
	intervalFactory        IntervalFactory
	errorAggregatorFactory ErrorAggregatorFactory
	contextOps             utilcontext.Ops
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

// NewOps returns the new ops with the given IntervalFactory, ErrorAggregatorFactory and utilcontext.Ops.
func NewOps(intervalFactory IntervalFactory, errorAggregatorFactory ErrorAggregatorFactory, contextOps utilcontext.Ops) Ops {
	return &ops{intervalFactory, errorAggregatorFactory, contextOps}
}

var defaultOps = NewOps(DefaultIntervalFactory(), DefaultErrorAggregatorFactory(), utilcontext.DefaultOps())

// DefaultOps returns the default Ops with the DefaultIntervalFactory, DefaultErrorAggregatorFactory and utilcontext.DefaultOps.
func DefaultOps() Ops {
	return defaultOps
}
