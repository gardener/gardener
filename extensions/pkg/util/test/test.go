// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
// ```
// v := "foo"
// defer WithVar(&v, "bar")()
// ```
func WithVar(dst interface{}, src interface{}) func() {
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
