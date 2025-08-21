// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fake

import (
	"github.com/gardener/gardener/pkg/webhook/authorizer"
)

type fakeWithSelectorsChecker struct {
	isPossible bool
}

var _ authorizer.WithSelectorsChecker = &fakeWithSelectorsChecker{}

// NewWithSelectorsChecker creates a new fake WithSelectorsChecker that already returns the provided value for the
// 'IsPossible' method.
func NewWithSelectorsChecker(isPossible bool) *fakeWithSelectorsChecker {
	return &fakeWithSelectorsChecker{isPossible: isPossible}
}

func (f *fakeWithSelectorsChecker) IsPossible() (bool, error) {
	return f.isPossible, nil
}
