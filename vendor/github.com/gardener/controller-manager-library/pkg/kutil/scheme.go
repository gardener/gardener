/*
 * Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 *
 */

package kutil

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
)

func IsListType(t reflect.Type) (reflect.Type, bool) {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	field, ok := t.FieldByName("Items")
	if !ok {
		return nil, false
	}

	t = field.Type
	if t.Kind() != reflect.Slice {
		return nil, false
	}
	t = t.Elem()
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, false
	}
	return t, true
}

var unstructuredType = reflect.TypeOf(unstructured.Unstructured{})
var unstructuredListType = reflect.TypeOf(unstructured.UnstructuredList{})

func DetermineListType(s *runtime.Scheme, gv schema.GroupVersion, t reflect.Type) reflect.Type {
	if t == unstructuredType {
		return unstructuredListType
	}
	for _gvk, _t := range s.AllKnownTypes() {
		if gv == _gvk.GroupVersion() {
			e, ok := IsListType(_t)
			if ok && e == t {
				return _t
			}
		}
	}
	return nil
}
