// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"fmt"

	"go.uber.org/mock/gomock"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HasObjectKeyOf returns a gomock.Matcher that matches if actual is a client.Object that has the same
// ObjectKey as expected.
func HasObjectKeyOf(expected client.Object) gomock.Matcher {
	return &objectKeyMatcher{key: client.ObjectKeyFromObject(expected)}
}

type objectKeyMatcher struct {
	key client.ObjectKey
}

func (o *objectKeyMatcher) Matches(actual any) bool {
	if actual == nil {
		return false
	}

	obj, ok := actual.(client.Object)
	if !ok {
		return false
	}

	return o.key == client.ObjectKeyFromObject(obj)
}

func (o *objectKeyMatcher) String() string {
	return fmt.Sprintf("has object key %q", o.key)
}
