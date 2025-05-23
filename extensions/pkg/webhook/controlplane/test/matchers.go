// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"fmt"
	"slices"
	"strings"

	"github.com/onsi/gomega/types"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
)

// ContainElementWithPrefixContaining succeeds if actual contains a string having the given prefix
// and containing the given value in a list separated by sep.
// Actual must be a slice of strings.
func ContainElementWithPrefixContaining(prefix, value, sep string) types.GomegaMatcher {
	return &containElementWithPrefixContainingMatcher{
		prefix: prefix,
		value:  value,
		sep:    sep,
	}
}

type containElementWithPrefixContainingMatcher struct {
	prefix, value, sep string
}

func (m *containElementWithPrefixContainingMatcher) Match(actual any) (success bool, err error) {
	items, ok := actual.([]string)
	if !ok {
		return false, fmt.Errorf("ContainElementWithPrefixContaining matcher expects []string")
	}
	i := extensionswebhook.StringWithPrefixIndex(items, m.prefix)
	if i < 0 {
		return false, nil
	}
	values := strings.Split(strings.TrimPrefix(items[i], m.prefix), m.sep)
	return slices.Index(values, m.value) >= 0, nil
}

func (m *containElementWithPrefixContainingMatcher) FailureMessage(actual any) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto contain an element with prefix '%s' containing '%s'", actual, m.prefix, m.value)
}

func (m *containElementWithPrefixContainingMatcher) NegatedFailureMessage(actual any) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nnot to contain an element with prefix '%s' containing '%s'", actual, m.prefix, m.value)
}
