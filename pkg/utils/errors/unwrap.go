// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"errors"
)

// Unwrap unwraps and returns the root error. Multiple wrappings via `fmt.Errorf` implementations are properly taken into account.
func Unwrap(err error) error {
	var done bool

	for !done {
		if err == nil || errors.Unwrap(err) == nil {
			// this most likely is the root error
			done = true
		} else {
			err = errors.Unwrap(err)
			done = false
		}
	}

	return err
}
