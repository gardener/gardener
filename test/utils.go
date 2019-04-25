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
	"github.com/onsi/ginkgo"
	"reflect"
)

// Must errors with `GinkgoT().Fatal` if the error is non-nil.
func Must(err error) {
	if err != nil {
		ginkgo.GinkgoT().Fatal(err)
	}
}

func checkPtr(v reflect.Value) error {
	if v.Type().Kind() != reflect.Ptr {
		return fmt.Errorf("value has to be a pointer-type but got %T", v.Interface())
	}
	return nil
}

func checkAssignable(src, dst reflect.Value) error {
	if !src.Type().AssignableTo(dst.Type().Elem()) {
		return fmt.Errorf("src of type %T cannot be assigned to dst of type %T", src.Interface(), dst.Interface())
	}
	return nil
}

func dereference(v interface{}) interface{} {
	dstValue := reflect.ValueOf(v)
	Must(checkPtr(dstValue))

	return dstValue.Elem().Interface()
}

// RevertableSet sets the element of dst to src and returns a function that can revert back to the original values.
func RevertableSet(dst, src interface{}) (revert func()) {
	tmp := dereference(dst)
	Set(dst, src)
	return func() { Set(dst, tmp) }
}

// Set sets the pointer dst to the value of src.
//
// dst has to be a pointer, src has to be assignable to the element type of dst.
func Set(dst, src interface{}) {
	dstValue := reflect.ValueOf(dst)
	Must(checkPtr(dstValue))

	srcValue := reflect.ValueOf(src)
	Must(checkAssignable(srcValue, dstValue))

	dstValue.Elem().Set(srcValue)
}
