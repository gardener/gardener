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

package gomega

import (
	"fmt"

	"github.com/onsi/gomega/format"
	"k8s.io/apimachinery/pkg/api/equality"
)

const (
	deepDerivativeMatcherNilError = `refusing to compare <nil> to <nil>.
Be explicit and use BeNil() instead.
This is to avoid mistakes where both sides of an assertion are erroneously uninitialized`
)

type deepDerivativeMatcher struct {
	expected interface{}
}

func (m *deepDerivativeMatcher) Match(actual interface{}) (success bool, err error) {
	if actual == nil && m.expected == nil {
		return false, fmt.Errorf(deepDerivativeMatcherNilError)
	}

	return equality.Semantic.DeepDerivative(m.expected, actual), nil
}

func (m *deepDerivativeMatcher) FailureMessage(actual interface{}) (message string) {
	actualString, actualOK := actual.(string)
	expectedString, expectedOK := m.expected.(string)

	if actualOK && expectedOK {
		return format.MessageWithDiff(actualString, "to deep derivative equal", expectedString)
	}

	return format.Message(actual, "to deep derivative equal", m.expected)
}

func (m *deepDerivativeMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to deep derivative equal", m.expected)
}
