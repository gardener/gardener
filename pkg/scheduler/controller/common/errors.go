// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package common

// AlreadyScheduledError is an error to indicate that a object is already scheduled when trying to update it.
type AlreadyScheduledError struct {
	msg string // description of error
}

func (e *AlreadyScheduledError) Error() string { return e.msg }

// NewAlreadyScheduledError creates new AlreadyScheduledError object with <message>.
func NewAlreadyScheduledError(message string) AlreadyScheduledError {
	return AlreadyScheduledError{
		msg: message,
	}
}
