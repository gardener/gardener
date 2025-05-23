// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers

import (
	"reflect"

	"github.com/onsi/gomega/format"
)

type referenceMatcher struct {
	expected any
}

func (r *referenceMatcher) Match(actual any) (success bool, err error) {
	return func(expected, actual any) bool {
		return reflect.ValueOf(expected).Pointer() == reflect.ValueOf(actual).Pointer()
	}(r.expected, actual), nil
}

func (r *referenceMatcher) FailureMessage(actual any) (message string) {
	return r.failureMessage(actual, "")
}

func (r *referenceMatcher) NegatedFailureMessage(actual any) (message string) {
	return r.failureMessage(actual, " not")
}

func (r *referenceMatcher) failureMessage(actual any, messagePrefix string) (message string) {
	return format.Message(actual, "to"+messagePrefix+" share reference with the compared object")
}
