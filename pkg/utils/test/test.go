// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"reflect"

	"github.com/onsi/ginkgo"
)

// WithVar sets the given var to the src value and returns a function to revert to the original state.
// The type of `dst` has to be a settable pointer.
// The value of `src` has to be assignable to the type of `dst`.
//
// Example usage:
//   v := "foo"
//   defer WithVar(&v, "bar")()
func WithVar(dst, src interface{}) func() {
	dstValue := reflect.ValueOf(dst)
	if dstValue.Type().Kind() != reflect.Ptr {
		ginkgo.Fail(fmt.Sprintf("destination value %T is not a pointer", dst))
	}

	if dstValue.CanSet() {
		ginkgo.Fail(fmt.Sprintf("value %T cannot be set", dst))
	}

	srcValue := reflect.ValueOf(src)
	if srcValue.Type().AssignableTo(dstValue.Type()) {
		ginkgo.Fail(fmt.Sprintf("cannot write %T into %T", src, dst))
	}

	tmp := dstValue.Elem().Interface()
	dstValue.Elem().Set(srcValue)
	return func() {
		dstValue.Elem().Set(reflect.ValueOf(tmp))
	}
}

// WithVars sets the given vars to the given values and returns a function to revert back.
// dstsAndSrcs have to appear in pairs of 2, otherwise there will be a runtime panic.
//
// Example usage:
//  defer WithVars(&v, "foo", &x, "bar")()
func WithVars(dstsAndSrcs ...interface{}) func() {
	if len(dstsAndSrcs)%2 != 0 {
		ginkgo.Fail(fmt.Sprintf("dsts and srcs are not of equal length: %v", dstsAndSrcs))
	}
	reverts := make([]func(), 0, len(dstsAndSrcs)/2)

	for i := 0; i < len(dstsAndSrcs); i += 2 {
		dst := dstsAndSrcs[i]
		src := dstsAndSrcs[i+1]

		reverts = append(reverts, WithVar(dst, src))
	}

	return func() {
		for _, revert := range reverts {
			revert()
		}
	}
}
