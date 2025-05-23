// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package retry

import (
	"context"
	"time"
)

// WaitFunc is a function that given context of a retry execution, returns a context that is closed
// after a predefined wait amount.
type WaitFunc func(context.Context) (context.Context, context.CancelFunc)

// Func is a function that can be retried.
//
// There four possible return combinations. For each of these, there's also a utility function:
// * Ok (true, nil): Execution succeeded without error.
// * NotOk (false, nil): Execution failed without error, can be retried.
// * MinorError (false, err): Execution failed with error, can be retried.
// * SevereError (true, err): Execution failed with error, cannot be retried.
type Func func(ctx context.Context) (done bool, err error)

// IntervalFactory is a factory that can produce WaitFuncs that wait for the given interval.
type IntervalFactory interface {
	New(interval time.Duration) WaitFunc
}

// IntervalFactoryFunc is a function that implements IntervalFactory.
type IntervalFactoryFunc func(interval time.Duration) WaitFunc

// Ops are additional operations that can be done based on the UntilFor method.
type Ops interface {
	// Until keeps retrying the given Func until it either errors severely or the context expires.
	// Between each try, it waits for the given interval.
	Until(ctx context.Context, interval time.Duration, f Func) error
	// UntilTimeout keeps retrying the given Func until it either errors severely or the context expires.
	// Between each try, it waits for the given interval.
	// It also passes down a modified context to the execution that times out after the given timeout.
	UntilTimeout(ctx context.Context, interval, timeout time.Duration, f Func) error
}

// An ErrorAggregator aggregates minor and severe errors.
//
// It's completely up to the ErrorAggregator how to aggregate the errors. Some may choose to only
// keep the most recent error they received.
// If no error was being recorded and the Error function is being called, the ErrorAggregator
// should return a proper zero value (in most cases, this will be nil).
type ErrorAggregator interface {
	// Minor records the given minor error.
	Minor(err error)
	// Severe records the given severe error.
	Severe(err error)
	// Error returns the aggregated error.
	Error() error
}

// ErrorAggregatorFactory is a factory that produces ErrorAggregators.
type ErrorAggregatorFactory interface {
	New() ErrorAggregator
}

// ErrorAggregatorFactoryFunc is a function that implements ErrorAggregatorFactory.
type ErrorAggregatorFactoryFunc func() ErrorAggregator
