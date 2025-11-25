// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"errors"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
)

// ErrorWithCodes contains the error and Gardener error codes.
type ErrorWithCodes struct {
	err   error
	codes []gardencorev1beta1.ErrorCode
}

// Retriable marks ErrorWithCodes as retriable.
func (e *ErrorWithCodes) Retriable() {}

// NewErrorWithCodes creates a new error that additionally exposes the given codes via the Coder interface.
func NewErrorWithCodes(err error, codes ...gardencorev1beta1.ErrorCode) error {
	return &ErrorWithCodes{err, codes}
}

// Codes returns all error codes.
func (e *ErrorWithCodes) Codes() []gardencorev1beta1.ErrorCode {
	return e.codes
}

// Unwrap rettieves the error from ErrorWithCodes.
func (e *ErrorWithCodes) Unwrap() error {
	return e.err
}

// Error returns the error message.
func (e *ErrorWithCodes) Error() string {
	return e.err.Error()
}

// Coder is an error that may produce a ErrorCodes visible to the outside.
type Coder interface {
	error
	Codes() []gardencorev1beta1.ErrorCode
}

// ExtractErrorCodes extracts all error codes from the given error by using errorsutils.Errors
func ExtractErrorCodes(err error) []gardencorev1beta1.ErrorCode {
	var codes []gardencorev1beta1.ErrorCode

	for _, err := range errorsutils.Errors(err) {
		var coder Coder
		if errors.As(err, &coder) {
			codes = append(codes, coder.Codes()...)
		}
	}
	return codes
}

var _ error = (*MultiErrorWithCodes)(nil)

// MultiErrorWithCodes is a struct that contains multiple errors and ErrorCodes.
type MultiErrorWithCodes struct {
	errors      []error
	errorFormat func(errs []error) string

	errorCodeStr sets.Set[string]
	codes        []gardencorev1beta1.ErrorCode
}

// NewMultiErrorWithCodes returns a new instance of `MultiErrorWithCodes`.
func NewMultiErrorWithCodes(errorFormat func(errs []error) string) *MultiErrorWithCodes {
	return &MultiErrorWithCodes{
		errorFormat:  errorFormat,
		errorCodeStr: sets.New[string](),
	}
}

// Append appends the given error to the `MultiErrorWithCodes`.
func (m *MultiErrorWithCodes) Append(err error) {
	for _, code := range ExtractErrorCodes(err) {
		if m.errorCodeStr.Has(string(code)) {
			continue
		}

		m.errorCodeStr.Insert(string(code))
		m.codes = append(m.codes, code)
	}

	m.errors = append(m.errors, err)
}

// Codes returns all underlying `gardencorev1beta1.ErrorCode` codes.
func (m *MultiErrorWithCodes) Codes() []gardencorev1beta1.ErrorCode {
	if m.codes == nil {
		return nil
	}

	cp := make([]gardencorev1beta1.ErrorCode, len(m.codes))
	copy(cp, m.codes)
	return cp
}

// ErrorOrNil returns nil if no underlying errors are given.
func (m *MultiErrorWithCodes) ErrorOrNil() error {
	if len(m.errors) == 0 {
		return nil
	}
	return m
}

// Error implements the error interface.
func (m *MultiErrorWithCodes) Error() string {
	return m.errorFormat(m.errors)
}

// FormatLastErrDescription formats the error message string for the last occurred error.
func FormatLastErrDescription(err error) string {
	errString := err.Error()
	if len(errString) > 0 {
		errString = strings.ToUpper(string(errString[0])) + errString[1:]
	}
	return errString
}

// WrappedLastErrors is a structure which contains the general description of the lastErrors which occurred and an array of all lastErrors
type WrappedLastErrors struct {
	Description string
	LastErrors  []gardencorev1beta1.LastError
}

// NewWrappedLastErrors returns a list of last errors.
func NewWrappedLastErrors(description string, err error) *WrappedLastErrors {
	var lastErrors []gardencorev1beta1.LastError

	for _, partError := range errorsutils.Errors(err) {
		lastErrors = append(lastErrors, *LastErrorWithTaskID(
			partError.Error(),
			errorsutils.GetID(partError),
			ExtractErrorCodes(partError)...))
	}

	return &WrappedLastErrors{
		Description: description,
		LastErrors:  lastErrors,
	}
}

// LastError creates a new LastError with the given description, optional codes and sets timestamp when the error is lastly observed.
func LastError(description string, codes ...gardencorev1beta1.ErrorCode) *gardencorev1beta1.LastError {
	return &gardencorev1beta1.LastError{
		Description: description,
		Codes:       codes,
		LastUpdateTime: &metav1.Time{
			Time: time.Now(),
		},
	}
}

// LastErrorWithTaskID creates a new LastError with the given description, the ID of the task when the error occurred, optional codes and sets timestamp when the error is lastly observed.
func LastErrorWithTaskID(description string, taskID string, codes ...gardencorev1beta1.ErrorCode) *gardencorev1beta1.LastError {
	return &gardencorev1beta1.LastError{
		Description: description,
		Codes:       codes,
		TaskID:      &taskID,
		LastUpdateTime: &metav1.Time{
			Time: time.Now(),
		},
	}
}

// HasNonRetryableErrorCode returns true if at least one of given list of last errors has at least one error code that
// indicates that an automatic retry would not help fixing the problem.
func HasNonRetryableErrorCode(lastErrors ...gardencorev1beta1.LastError) bool {
	for _, lastError := range lastErrors {
		for _, code := range lastError.Codes {
			if code == gardencorev1beta1.ErrorInfraUnauthenticated ||
				code == gardencorev1beta1.ErrorInfraUnauthorized ||
				code == gardencorev1beta1.ErrorInfraDependencies ||
				code == gardencorev1beta1.ErrorInfraQuotaExceeded ||
				code == gardencorev1beta1.ErrorInfraRateLimitsExceeded ||
				code == gardencorev1beta1.ErrorConfigurationProblem ||
				code == gardencorev1beta1.ErrorProblematicWebhook {
				return true
			}
		}
	}
	return false
}

// HasErrorCode checks whether at least one LastError from the given slice of LastErrors <lastErrors>
// contains the given ErrorCode <code>.
func HasErrorCode(lastErrors []gardencorev1beta1.LastError, code gardencorev1beta1.ErrorCode) bool {
	for _, lastError := range lastErrors {
		for _, current := range lastError.Codes {
			if current == code {
				return true
			}
		}
	}

	return false
}
