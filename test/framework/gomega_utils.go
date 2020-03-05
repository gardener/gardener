// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package framework

import (
	"fmt"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/api/equality"
)

// ExpectNoError checks if an error has occurred
func ExpectNoError(actual interface{}, extra ...interface{}) {
	gomega.ExpectWithOffset(1, actual, extra...).ToNot(gomega.HaveOccurred())
}

// DeepDerivativeEqual is similar to DeepEqual except that unset fields in actual are
// ignored (not compared). This allows us to focus on the fields that matter to
// the semantic comparison.
func DeepDerivativeEqual(expected interface{}) types.GomegaMatcher {
	return &deepDerivativeMatcher{
		expected: expected,
	}
}

type deepDerivativeMatcher struct {
	expected interface{}
}

func (m *deepDerivativeMatcher) Match(actual interface{}) (success bool, err error) {
	if actual == nil && m.expected == nil {
		return false, fmt.Errorf("refusing to compare <nil> to <nil>.\nBe explicit and use BeNil() instead.  This is to avoid mistakes where both sides of an assertion are erroneously uninitialized")
	}
	return equality.Semantic.DeepDerivative(actual, m.expected), nil
}

func (m *deepDerivativeMatcher) FailureMessage(actual interface{}) (message string) {
	actualString, actualOK := actual.(string)
	expectedString, expectedOK := m.expected.(string)
	if actualOK && expectedOK {
		return format.MessageWithDiff(actualString, "to equal", expectedString)
	}

	return format.Message(actual, "to deep derivative equal", m.expected)
}

func (m *deepDerivativeMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to deep derivative equal", m.expected)
}
