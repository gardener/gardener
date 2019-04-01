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

package fieldpath

import (
	"reflect"
	"unicode"
)

func IsIdentifierStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func IsIdentifierPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func Value(val interface{}) interface{} {
	if val == nil {
		return nil
	}
	v := reflect.ValueOf(val)
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	return v.Interface()
}

////////////////////////////////////////////////////////////////////////////////

func valueType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func toValue(v reflect.Value, addMissing bool) reflect.Value {
	if isPtr(v) {
		if v.IsNil() {
			if addMissing {
				//fmt.Printf("CREATE %s\n", v.Type().Elem())
				v.Set(reflect.New(v.Type().Elem()))
			} else {
				//fmt.Print("NIL\n")
				return reflect.Value{}
			}
		}
		return v.Elem()
	}
	return v
}

func isPtr(v reflect.Value) bool {
	if !v.IsValid() {
		return false
	}
	return v.Kind() == reflect.Ptr
}
