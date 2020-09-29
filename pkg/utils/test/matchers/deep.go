// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	"github.com/onsi/gomega/format"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/yaml"
)

const (
	deepMatcherNilError = `refusing to compare <nil> to <nil>.
Be explicit and use BeNil() instead.
This is to avoid mistakes where both sides of an assertion are erroneously uninitialized`
)

type deepMatcher struct {
	name      string
	expected  interface{}
	compareFn func(a1, a2 interface{}) bool
}

func newDeepDerivativeMatcher(expected interface{}) gomegatypes.GomegaMatcher {
	return &deepMatcher{
		name:      "deep derivative equal",
		expected:  expected,
		compareFn: equality.Semantic.DeepDerivative,
	}
}

func newDeepEqualMatcher(expected interface{}) gomegatypes.GomegaMatcher {
	return &deepMatcher{
		name:      "deep equal",
		expected:  expected,
		compareFn: equality.Semantic.DeepEqual,
	}
}

func (m *deepMatcher) Match(actual interface{}) (success bool, err error) {
	if actual == nil && m.expected == nil {
		return false, fmt.Errorf(deepMatcherNilError)
	}

	return m.compareFn(m.expected, actual), nil
}

func (m *deepMatcher) FailureMessage(actual interface{}) (message string) {
	return m.failureMessage(actual, "to")
}

func (m *deepMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return m.failureMessage(actual, "not to")
}

func (m *deepMatcher) failureMessage(actual interface{}, messagePrefix string) (message string) {
	var (
		actualYAML, actualErr     = yaml.Marshal(actual)
		expectedYAML, expectedErr = yaml.Marshal(m.expected)
	)

	if actualErr == nil && expectedErr == nil {
		return format.MessageWithDiff(string(actualYAML), messagePrefix+" "+m.name, string(expectedYAML))
	}

	return format.Message(actual, messagePrefix+" "+m.name, m.expected)
}
