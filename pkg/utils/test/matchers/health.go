// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers

import (
	"fmt"

	"github.com/onsi/gomega/format"
	gomegatypes "github.com/onsi/gomega/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BeHealthy wraps a function (e.g., health.CheckDeployment) in a gomega matcher.
// If the given health function returns nil for the actual value, the matcher succeeds.
// If the health function returns an error, the matcher fails with the returned health check details.
// For example:
//
//	Expect(deployment).To(BeHealthy(health.CheckDeployment))
func BeHealthy[T client.Object](healthFunc func(T) error) gomegatypes.GomegaMatcher {
	return &healthMatcher[T]{
		healthFunc: healthFunc,
	}
}

type healthMatcher[T client.Object] struct {
	healthFunc func(T) error

	failure error
}

func (h *healthMatcher[T]) Match(actual any) (success bool, err error) {
	obj, ok := actual.(T)
	if !ok {
		return false, fmt.Errorf("expected %T but got %T", obj, actual)
	}

	h.failure = h.healthFunc(obj)
	return h.failure == nil, nil
}

func (h *healthMatcher[T]) FailureMessage(actual any) string {
	return h.expectedString(actual) +
		fmt.Sprintf("to be healthy but the health check returned the following error:\n%s", format.IndentString(h.failure.Error(), 1))
}

func (h *healthMatcher[T]) NegatedFailureMessage(actual any) (message string) {
	return h.expectedString(actual) + "not to be healthy but the health check did not return any error"
}

func (h *healthMatcher[T]) expectedString(actual any) string {
	obj := actual.(T)
	return fmt.Sprintf("Expected\n%s%T %s\n", format.Indent, obj, client.ObjectKeyFromObject(obj))
}
