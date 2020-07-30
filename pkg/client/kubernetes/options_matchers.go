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

package kubernetes

import (
	"fmt"
	"reflect"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/client-go/rest"
)

// ConsistOfConfigFuncs returns a composed `ConsistsOf` matcher with `MatchConfigFunc` for each ConfigFn in `fns`.
// This is useful for making assertions on a given slice of ConfigFns which can't be compared by value.
// e.g.:
// 	Expect(fns).To(kubernetes.ConsistOfConfigFuncs(
//		kubernetes.WithClientConnectionOptions(clientConnectionConfig),
//		kubernetes.WithClientOptions(clientOptions),
//		kubernetes.WithDisabledCachedClient(),
//	))
func ConsistOfConfigFuncs(fns ...interface{}) gomegatypes.GomegaMatcher {
	var matchers []gomegatypes.GomegaMatcher

	for _, fn := range fns {
		matchers = append(matchers, MatchConfigFunc(fn))
	}

	return gomega.ConsistOf(matchers)
}

// MatchConfigFunc returns a matcher that checks if the config produced by the actual ConfigFn is deeply equal to the
// config produced by `fn`. This is useful for making assertions on given ConfigFns which can't be compared by value.
// e.g.:
// 	Expect(fn).Should(MatchConfigFunc(WithClientConnectionOptions(clientConnectionConfig)))
func MatchConfigFunc(fn interface{}) gomegatypes.GomegaMatcher {
	return &configFuncMatcher{expected: fn}
}

type configFuncMatcher struct {
	expected interface{}

	expectedConfig *config
	actualConfig   *config
}

func (m *configFuncMatcher) Match(actual interface{}) (success bool, err error) {
	if m.expected == nil {
		return false, fmt.Errorf("Refusing to compare <nil> to <nil>.\nBe explicit and use BeNil() instead.  This is to avoid mistakes where both sides of an assertion are erroneously uninitialized.")
	}
	if actual == nil {
		return false, nil
	}

	actualConfigFunc, ok := actual.(ConfigFunc)
	if !ok {
		return false, fmt.Errorf("actual is not a ConfigFunc, but %T", actual)
	}
	actualConfig := &config{restConfig: &rest.Config{}}
	if err := actualConfigFunc(actualConfig); err != nil {
		return false, fmt.Errorf("actual returned an error when calling: %w", err)
	}

	expectedConfigFunc, ok := m.expected.(ConfigFunc)
	if !ok {
		return false, fmt.Errorf("expected is not a ConfigFunc, but %T", m.expected)
	}
	expectedConfig := &config{restConfig: &rest.Config{}}
	if err := expectedConfigFunc(expectedConfig); err != nil {
		return false, fmt.Errorf("expected returned an error when calling: %w", err)
	}

	return reflect.DeepEqual(expectedConfig, actualConfig), nil
}

func (m *configFuncMatcher) FailureMessage(actual interface{}) (message string) {
	if m.actualConfig == nil || m.expectedConfig == nil {
		return format.Message(actual, "to produce an equal config to the one produced by", m.expected)
	}

	return format.MessageWithDiff(fmt.Sprintf("%+v", m.actualConfig), "to produce an equal config to the one produced by", fmt.Sprintf("%+v", m.expectedConfig))
}

func (m *configFuncMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	if m.actualConfig == nil || m.expectedConfig == nil {
		return format.Message(actual, "to not produce an equal config to the one produced by", m.expected)
	}

	return format.MessageWithDiff(fmt.Sprintf("%+v", m.actualConfig), "to not produce an equal config to the one produced by", fmt.Sprintf("%+v", m.expectedConfig))
}
