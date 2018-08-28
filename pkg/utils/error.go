package utils

import "github.com/hashicorp/go-multierror"

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
