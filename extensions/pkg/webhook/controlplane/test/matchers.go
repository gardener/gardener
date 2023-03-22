// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package test

import (
	"fmt"
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

func (m *containElementWithPrefixContainingMatcher) Match(actual interface{}) (success bool, err error) {
	items, ok := actual.([]string)
	if !ok {
		return false, fmt.Errorf("ContainElementWithPrefixContaining matcher expects []string")
	}
	i := extensionswebhook.StringWithPrefixIndex(items, m.prefix)
	if i < 0 {
		return false, nil
	}
	values := strings.Split(strings.TrimPrefix(items[i], m.prefix), m.sep)
	j := extensionswebhook.StringIndex(values, m.value)
	return j >= 0, nil
}

func (m *containElementWithPrefixContainingMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto contain an element with prefix '%s' containing '%s'", actual, m.prefix, m.value)
}

func (m *containElementWithPrefixContainingMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nnot to contain an element with prefix '%s' containing '%s'", actual, m.prefix, m.value)
}
