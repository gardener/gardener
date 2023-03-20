// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/golang/mock/gomock"
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

func (o *objectKeyMatcher) Matches(actual interface{}) bool {
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
