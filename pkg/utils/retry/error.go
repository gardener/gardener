// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package retry

import (
	"errors"
)

// retriableError is a marker interface indicating that an error occurred in a different component occurred (e.g. during
// reconciliation of an extension resource, the extension controller sets status.lastError), but the error should not be
// treated as "severe" immediately. Instead, the wait func is supposed to continue retrying waiting for the health
// condition to be met and only treat the error as severe, if it persists.
type retriableError interface {
	error
	// Retriable distinguishes an retriableError from other errors.
	Retriable()
}

// IsRetriable checks if any error in err's chain is marked as an retriableError.
func IsRetriable(err error) bool {
	var r retriableError
	return errors.As(err, &r)
}

// RetriableError marks a given error as retriable.
func RetriableError(err error) error {
	return retriableErrorImpl{underlying: err}
}

var _ retriableError = retriableErrorImpl{}

type retriableErrorImpl struct {
	underlying error
}

// Error return the error message of the underlying (wrapped) error.
func (r retriableErrorImpl) Error() string {
	return r.underlying.Error()
}

// Retriable marks retriableErrorImpl as retriable.
func (r retriableErrorImpl) Retriable() {}

// Unwrap returns the underlying (wrapped) error.
func (r retriableErrorImpl) Unwrap() error {
	return r.underlying
}
