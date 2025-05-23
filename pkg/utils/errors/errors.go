// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"fmt"
	"io"

	"github.com/hashicorp/go-multierror"
)

type withSuppressed struct {
	cause      error
	suppressed error
}

func (w *withSuppressed) Error() string {
	return fmt.Sprintf("%s, suppressed: %s", w.cause.Error(), w.suppressed.Error())
}

func (w *withSuppressed) Unwrap() error {
	return w.cause
}

func (w *withSuppressed) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = fmt.Fprintf(s, "%+v\nsuppressed: %+v", w.Unwrap(), w.suppressed)
			return
		}

		fallthrough
	case 's', 'q':
		_, _ = io.WriteString(s, w.Error())
	}
}

func (w *withSuppressed) Suppressed() error {
	return w.suppressed
}

// Suppressed retrieves the suppressed error of the given error, if any.
// An error has a suppressed error if it implements the following interface:
//
//	type suppressor interface {
//	       Suppressed() error
//	}
//
// If the error does not implement the interface, nil is returned.
func Suppressed(err error) error {
	type suppressor interface {
		Suppressed() error
	}
	if w, ok := err.(suppressor); ok {
		return w.Suppressed()
	}
	return nil
}

// WithSuppressed annotates err with a suppressed error.
// If err is nil, WithSuppressed returns nil.
// If suppressed is nil, WithSuppressed returns err.
func WithSuppressed(err, suppressed error) error {
	if err == nil || suppressed == nil {
		return err
	}

	return &withSuppressed{
		cause:      err,
		suppressed: suppressed,
	}
}

// reconciliationError implements ErrorIDer
type reconciliationError struct {
	error
	errorID string
}

// WithID annotates the error with the given errorID which can afterwards be retrieved by ErrorID()
func WithID(id string, err error) error {
	return &reconciliationError{err, id}
}

// ErrorID implements the errorIDer interface and returns the id of the reconciliationError
func (t *reconciliationError) ErrorID() string {
	return t.errorID
}

func (t *reconciliationError) Unwrap() error {
	return t.error
}

// GetID returns the ID of the error if possible.
// If err does not implement ErrorID or is nil an empty string will be returned.
func GetID(err error) string {
	type errorIDer interface {
		ErrorID() string
	}

	var id string

	if err != nil {
		if errWithID, ok := err.(errorIDer); ok {
			id = errWithID.ErrorID()
		}
	}
	return id
}

// The ErrorContext holds the lastError IDs from the previous reconciliaton and the IDs of the errors that are processed in this context during the current reconciliation
type ErrorContext struct {
	name         string
	lastErrorIDs []string
	errorIDs     map[string]struct{}
}

// NewErrorContext creates a new error context with the given name and lastErrors from the previous reconciliation
func NewErrorContext(name string, lastErrorIDs []string) *ErrorContext {
	return &ErrorContext{
		name:         name,
		lastErrorIDs: lastErrorIDs,
		errorIDs:     map[string]struct{}{},
	}
}

// AddErrorID adds an error ID which will be tracked by the context and panics if more than one error have the same ID
func (e *ErrorContext) AddErrorID(errorID string) {
	if e.HasErrorWithID(errorID) {
		panic(fmt.Sprintf("Error with id %q already exists in error context %q", errorID, e.name))
	}
	e.errorIDs[errorID] = struct{}{}
}

// HasErrorWithID checks if the ErrorContext already contains an error with id errorID
func (e *ErrorContext) HasErrorWithID(errorID string) bool {
	_, ok := e.errorIDs[errorID]
	return ok
}

// HasLastErrorWithID checks if the previous reconciliation had encountered an error with id errorID
func (e *ErrorContext) HasLastErrorWithID(errorID string) bool {
	for _, lastErrorID := range e.lastErrorIDs {
		if errorID == lastErrorID {
			return true
		}
	}
	return false
}

// FailureHandler is a function which is called when an error occurs
type FailureHandler func(string, error) error

// SuccessHandler is called when a task completes successfully
type SuccessHandler func(string) error

// TaskFunc is an interface for a task which should belong to an ErrorContext and can trigger OnSuccess and OnFailure callbacks depending on whether it completes successfully or not
type TaskFunc interface {
	Do(errorContext *ErrorContext) (string, error)
}

// taskFunc implements TaskFunc
type taskFunc func(*ErrorContext) (string, error)

func (f taskFunc) Do(errorContext *ErrorContext) (string, error) {
	return f(errorContext)
}

func defaultFailureHandler(errorID string, err error) error {
	err = fmt.Errorf("%s failed (%v)", errorID, err)
	return WithID(errorID, err)
}

// ToExecute takes an errorID and a function and creates a TaskFunc from them.
func ToExecute(errorID string, task func() error) TaskFunc {
	return taskFunc(func(errorContext *ErrorContext) (string, error) {
		errorContext.AddErrorID(errorID)
		err := task()
		if err != nil {
			return errorID, err
		}
		return errorID, nil
	})
}

// HandleErrors takes a reference to an ErrorContext, onSuccess and onFailure callback functions and a variadic list of taskFuncs.
// It sequentially adds the Tasks' errorIDs to the provided ErrorContext and executes them.
// If the ErrorContext has errors from the previous reconciliation and the tasks which caused errors complete successfully OnSuccess is called.
// If a task fails OnFailure is called
func HandleErrors(errorContext *ErrorContext, onSuccess SuccessHandler, onFailure FailureHandler, tasks ...TaskFunc) error {
	for _, task := range tasks {
		errorID, err := task.Do(errorContext)
		if err != nil {
			return handleFailure(onFailure, errorID, err)
		}
		if handlerErr := handleSuccess(errorContext, onSuccess, errorID); handlerErr != nil {
			return handlerErr
		}
	}
	return nil
}

func handleFailure(onFailure FailureHandler, errorID string, err error) error {
	if onFailure != nil {
		return onFailure(errorID, err)
	}
	return defaultFailureHandler(errorID, err)
}

func handleSuccess(errorContext *ErrorContext, onSuccess SuccessHandler, errorID string) error {
	if onSuccess != nil && errorContext.HasLastErrorWithID(errorID) {
		if err := onSuccess(errorID); err != nil {
			return err
		}
	}
	return nil
}

// Errors returns a list of all nested errors of the given error.
// If the error is nil, nil is returned.
// If the error is a multierror, it returns all its errors.
// Otherwise, it returns a slice containing the error as single element.
func Errors(err error) []error {
	if err == nil {
		return nil
	}
	if errs, ok := err.(*multierror.Error); ok {
		return errs.Errors
	}
	return []error{err}
}
