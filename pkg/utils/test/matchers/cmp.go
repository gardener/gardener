// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package matchers

import (
	"bytes"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

// Inspired by https://github.com/onsi/gomega/pull/546
// TODO(ScheererJ): Should be removed once gomega with BeComparableTo matcher is available

//BeComparableTo uses gocmp.Equal to compare. You can pass cmp.Option as options.
//It is an error for actual and expected to be nil.  Use BeNil() instead.
func BeComparableTo(expected interface{}, opts ...cmp.Option) types.GomegaMatcher {
	return &BeComparableToMatcher{
		Expected: expected,
		Options:  opts,
	}
}

// BeComparableToMatcher allows comparisons with additional comparison options
type BeComparableToMatcher struct {
	Expected interface{}
	Options  cmp.Options
}

// Match compares the actual with the expected value
func (matcher *BeComparableToMatcher) Match(actual interface{}) (success bool, matchErr error) {
	if actual == nil && matcher.Expected == nil {
		return false, fmt.Errorf("refusing to compare <nil> to <nil>.\nBe explicit and use BeNil() instead.  This is to avoid mistakes where both sides of an assertion are erroneously uninitialized")
	}
	// Shortcut for byte slices.
	// Comparing long byte slices with reflect.DeepEqual is very slow,
	// so use bytes.Equal if actual and expected are both byte slices.
	if actualByteSlice, ok := actual.([]byte); ok {
		if expectedByteSlice, ok := matcher.Expected.([]byte); ok {
			return bytes.Equal(actualByteSlice, expectedByteSlice), nil
		}
	}

	defer func() {
		if r := recover(); r != nil {
			success = false
			if err, ok := r.(error); ok {
				matchErr = err
			} else if errMsg, ok := r.(string); ok {
				matchErr = fmt.Errorf(errMsg)
			}
		}
	}()

	return cmp.Equal(actual, matcher.Expected, matcher.Options...), nil
}

// FailureMessage returns the message of the failure
func (matcher *BeComparableToMatcher) FailureMessage(actual interface{}) (message string) {
	return cmp.Diff(matcher.Expected, actual)
}

// NegatedFailureMessage returns the negated message of the failure
func (matcher *BeComparableToMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to equal", matcher.Expected)
}
