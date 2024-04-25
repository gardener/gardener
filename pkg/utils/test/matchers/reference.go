// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers

import (
	"reflect"

	"github.com/onsi/gomega/format"
)

type referenceMatcher struct {
	expected interface{}
}

func (r *referenceMatcher) Match(actual interface{}) (success bool, err error) {
	return func(expected, actual interface{}) bool {
		return reflect.ValueOf(expected).Pointer() == reflect.ValueOf(actual).Pointer()
	}(r.expected, actual), nil
}

func (r *referenceMatcher) FailureMessage(actual interface{}) (message string) {
	return r.failureMessage(actual, "")
}

func (r *referenceMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return r.failureMessage(actual, " not")
}

func (r *referenceMatcher) failureMessage(actual interface{}, messagePrefix string) (message string) {
	return format.Message(actual, "to"+messagePrefix+" share reference with the compared object")
}
