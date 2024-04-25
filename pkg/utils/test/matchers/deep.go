// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers

import (
	"errors"

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
		return false, errors.New(deepMatcherNilError)
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
