// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"fmt"
	"reflect"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	gomegatypes "github.com/onsi/gomega/types"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// ConsistOfConfigFuncs returns a composed `ConsistsOf` matcher with `MatchConfigFunc` for each ConfigFn in `fns`.
// This is useful for making assertions on a given slice of ConfigFns which can't be compared by value.
// e.g.:
//
//	Expect(fns).To(kubernetes.ConsistOfConfigFuncs(
//		kubernetes.WithClientConnectionOptions(clientConnectionConfig),
//		kubernetes.WithClientOptions(clientOptions),
//		kubernetes.WithDisabledCachedClient(),
//	))
func ConsistOfConfigFuncs(fns ...any) gomegatypes.GomegaMatcher {
	var matchers []gomegatypes.GomegaMatcher

	for _, fn := range fns {
		matchers = append(matchers, MatchConfigFunc(fn))
	}

	return gomega.ConsistOf(matchers)
}

// MatchConfigFunc returns a matcher that checks if the config produced by the actual ConfigFn is deeply equal to the
// config produced by `fn`. This is useful for making assertions on given ConfigFns which can't be compared by value.
// e.g.:
//
//	Expect(fn).Should(MatchConfigFunc(WithClientConnectionOptions(clientConnectionConfig)))
func MatchConfigFunc(fn any) gomegatypes.GomegaMatcher {
	return &configFuncMatcher{expected: fn}
}

type configFuncMatcher struct {
	expected any

	expectedConfig, actualConfig *kubernetes.Config
}

func (m *configFuncMatcher) Match(actual any) (success bool, err error) {
	if m.expected == nil {
		return false, fmt.Errorf("Refusing to compare <nil> to <nil>.\nBe explicit and use BeNil() instead.  This is to avoid mistakes where both sides of an assertion are erroneously uninitialized.") //nolint:revive,staticcheck
	}
	if actual == nil {
		return false, nil
	}

	actualConfigFunc, ok := actual.(kubernetes.ConfigFunc)
	if !ok {
		return false, fmt.Errorf("actual is not a ConfigFunc, but %T", actual)
	}
	m.actualConfig = kubernetes.NewConfig()
	if err := actualConfigFunc(m.actualConfig); err != nil {
		return false, fmt.Errorf("actual returned an error when calling: %w", err)
	}

	expectedConfigFunc, ok := m.expected.(kubernetes.ConfigFunc)
	if !ok {
		return false, fmt.Errorf("expected is not a ConfigFunc, but %T", m.expected)
	}
	m.expectedConfig = kubernetes.NewConfig()
	if err := expectedConfigFunc(m.expectedConfig); err != nil {
		return false, fmt.Errorf("expected returned an error when calling: %w", err)
	}

	return reflect.DeepEqual(m.expectedConfig, m.actualConfig), nil
}

func (m *configFuncMatcher) FailureMessage(actual any) (message string) {
	if m.actualConfig == nil || m.expectedConfig == nil {
		return format.Message(actual, "to produce an equal config to the one produced by", m.expected)
	}

	return format.MessageWithDiff(fmt.Sprintf("%+v", m.actualConfig), "to produce an equal config to the one produced by", fmt.Sprintf("%+v", m.expectedConfig))
}

func (m *configFuncMatcher) NegatedFailureMessage(actual any) (message string) {
	if m.actualConfig == nil || m.expectedConfig == nil {
		return format.Message(actual, "to not produce an equal config to the one produced by", m.expected)
	}

	return format.MessageWithDiff(fmt.Sprintf("%+v", m.actualConfig), "to not produce an equal config to the one produced by", fmt.Sprintf("%+v", m.expectedConfig))
}
