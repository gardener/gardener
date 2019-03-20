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
	"fmt"
	"reflect"
)

type Field interface {
	BaseType() reflect.Type
	Type() reflect.Type
	Get(base interface{}) (interface{}, error)
	GetAsValue(base interface{}) (interface{}, error)
	Set(base interface{}, value interface{}) error

	String() string
}

type field struct {
	node      Node
	baseType  reflect.Type
	fieldType reflect.Type
}

var _ Field = &field{}

func NewField(base interface{}, path string) (Field, error) {
	n, err := Compile(path)
	if err != nil {
		return nil, err
	}
	t, err := n.Type(base)
	if err != nil {
		return nil, err
	}
	return &field{n, valueType(reflect.TypeOf(base)), t}, nil
}

func RequiredField(base interface{}, path string) Field {
	f, err := NewField(base, path)
	if err != nil {
		panic(fmt.Sprintf("FieldNode %q for %T is invalid: %s", path, base, err))
	}
	return f
}

func (this *field) String() string {
	return fmt.Sprintf("%s%s", this.baseType, this.node)
}

func (this *field) Type() reflect.Type {
	return this.fieldType
}
func (this *field) BaseType() reflect.Type {
	return this.baseType
}
func (this *field) Get(base interface{}) (interface{}, error) {
	if valueType(reflect.TypeOf(base)) != this.baseType {
		return nil, fmt.Errorf("invalid base element: got %T, expected %s", base, this.baseType)
	}
	return this.node.Get(base)
}
func (this *field) GetAsValue(base interface{}) (interface{}, error) {
	v, err := this.Get(base)
	if err != nil {
		return nil, err
	}
	value := reflect.ValueOf(v)
	if value.IsNil() {
		return nil, nil
	}
	for value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	return value.Interface(), nil
}

func (this *field) Set(base interface{}, value interface{}) error {
	if reflect.TypeOf(base) != reflect.PtrTo(this.baseType) {
		return fmt.Errorf("invalid base element: got %T, expected %s", base, reflect.PtrTo(this.baseType))
	}
	return this.node.Set(base, value)
}
