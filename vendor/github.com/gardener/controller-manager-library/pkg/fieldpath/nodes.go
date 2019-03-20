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

type Node interface {
	Next() Node
	String() string

	Type(interface{}) (reflect.Type, error)
	Validate(interface{}) error
	ValidateType(interface{}, interface{}) error

	Get(interface{}) (interface{}, error)
	Set(interface{}, interface{}) error

	_value(src reflect.Value, addMissing bool) (reflect.Value, error)
	value(src reflect.Value, addMissing bool) (reflect.Value, error)
}

type new interface {
	new(self, next Node) Node
}

type node struct {
	next Node
	self Node
}

func (this *node) new(self, next Node) Node {
	this.self = self
	this.next = next
	return self
}

func (this *node) Next() Node {
	return this.next
}

func (this *node) String() string {
	if this.next == nil {
		return ""
	}
	return this.next.String()
}

func (this *node) Type(src interface{}) (reflect.Type, error) {
	v, ok := src.(reflect.Value)
	if ok {
		return this._type(v)
	}
	return this._type(reflect.ValueOf(src))
}

func (this *node) _type(v reflect.Value) (reflect.Type, error) {
	field, err := this._value(v, true)
	if err != nil {
		return nil, err
	}
	return field.Type(), nil
}

func (this *node) Validate(src interface{}) error {
	v, ok := src.(reflect.Value)
	if ok {
		return this._validate(v)
	}
	return this._validate(reflect.ValueOf(src))
}

func (this *node) ValidateType(src interface{}, val interface{}) error {
	v, ok := src.(reflect.Value)
	if !ok {
		v = reflect.ValueOf(src)
	}
	t, ok := val.(reflect.Type)
	if !ok {
		t = reflect.TypeOf(val)
	}
	return this._validateType(v, t)
}

func (this *node) Get(src interface{}) (interface{}, error) {
	v, ok := src.(reflect.Value)
	if ok {
		return this._get(v)
	}
	return this._get(reflect.ValueOf(src))
}

func (this *node) Set(src interface{}, val interface{}) error {
	v, ok := src.(reflect.Value)
	if ok {
		return this._set(v, val)
	}
	return this._set(reflect.ValueOf(src), val)
}

func (this *node) _value(v reflect.Value, addMissing bool) (reflect.Value, error) {
	var err error

	//fmt.Printf("value: %s\n", this.self.String())
	if this.next != nil {
		v, err = this.next._value(v, addMissing)
		if err != nil {
			return v, err
		}
	}
	return this.self.value(v, addMissing)
}

func (this *node) _validate(v reflect.Value) error {

	_, err := this._value(v, false)
	return err
}

func (this *node) _validateType(v reflect.Value, vtype reflect.Type) error {

	field, err := this._value(v, false)
	if err != nil {
		return err
	}
	ftype := field.Type()
	if ftype == vtype {
		return nil
	}

	if ftype.Kind() == reflect.Ptr {
		ftype = ftype.Elem()
		if ftype == vtype {
			return nil
		}
	}
	return fmt.Errorf("%q is not assignable from %q", field.Type(), vtype)
}

func (this *node) _get(v reflect.Value) (interface{}, error) {

	field, err := this._value(v, false)
	if err != nil {
		return nil, err
	}
	return field.Interface(), nil
}

func (this *node) _set(v reflect.Value, val interface{}) error {

	field, err := this._value(v, true)
	if err != nil {
		return err
	}

	a := reflect.ValueOf(val)
	//fmt.Printf("assign %s: %s from %T(%#v)\n", this.self.String(), field.Type(), val, val)

	if val == nil {
		k := v.Kind()
		if k != reflect.Ptr &&
			k != reflect.Slice &&
			k != reflect.Map &&
			k != reflect.Func &&
			k != reflect.Chan &&
			k != reflect.Interface {
			return fmt.Errorf("nil not asignable to %q", v.Type())
		}
		a = reflect.Zero(field.Type())
	} else {
		if field.Kind() == reflect.Ptr && a.Kind() != reflect.Ptr {
			p := reflect.New(a.Type())
			p.Elem().Set(a)
			a = p
		}
		if !a.Type().AssignableTo(field.Type()) {
			return fmt.Errorf("%q not asignable to %q", a.Type(), field.Type())
		}
	}
	field.Set(a)

	return nil
}

////////////////////////////////////////////////////////////////////////////////

type FieldNode struct {
	node
	name string
}

var _ Node = &FieldNode{}

func NewFieldNode(name string, next Node) Node {
	f := &FieldNode{name: name}

	return f.new(f, next)
}

func (this *FieldNode) String() string {
	return fmt.Sprintf("%s.%s", this.node.String(), this.name)
}

func (this *FieldNode) value(v reflect.Value, addMissing bool) (reflect.Value, error) {
	v = toValue(v, addMissing)
	if !v.IsValid() {
		return reflect.Value{}, fmt.Errorf("%s is <nil>", this.node.String())
	}
	if v.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("%s is no struct", this.node.String())
	}
	//fmt.Printf("TYPE %s: %s lookup %s\n", this.node.String(), v.Type(), this.name)
	field := v.FieldByName(this.name)
	if !field.IsValid() {
		return reflect.Value{}, fmt.Errorf("%s has no field %q", this.node.String(), this.name)
	}
	return field, nil
}

////////////////////////////////////////////////////////////////////////////////

type SliceEntryNode struct {
	node
	index int
}

var _ Node = &SliceEntryNode{}

func NewEntry(index int, next Node) Node {
	e := &SliceEntryNode{index: index}
	return e.new(e, next)
}

func (this *SliceEntryNode) String() string {
	return fmt.Sprintf("%s[%d]", this.node.String(), this.index)
}

func (this *SliceEntryNode) value(v reflect.Value, addMissing bool) (reflect.Value, error) {
	v = toValue(v, addMissing)
	if v.Kind() != reflect.Array && v.Kind() != reflect.Slice {
		return reflect.Value{}, fmt.Errorf("%s is no slice or array(%s) ", this.node.String(), v.Type())
	}
	if v.Len() <= this.index {
		if !addMissing || v.Kind() == reflect.Array {
			return reflect.Value{}, fmt.Errorf("%s has size %d, but expected at least %d", this.node.String(), v.Len(), this.index+1)
		}
		e := reflect.New(v.Type().Elem())
		for v.Len() <= this.index {
			//fmt.Printf("APPEND %d\n", v.Len())
			v.Set(reflect.Append(v, e.Elem()))
		}
	}
	return v.Index(this.index), nil
}

////////////////////////////////////////////////////////////////////////////////

type SelectionNode struct {
	node
	path  Node
	match interface{}
}

var _ Node = &SelectionNode{}

func NewSelection(path Node, value interface{}, next Node) Node {
	e := &SelectionNode{path: path, match: value}
	return e.new(e, next)
}

func (this *SelectionNode) String() string {
	vs := ""
	switch this.match.(type) {
	case int:
		vs = fmt.Sprintf("%d", this.match)
	case string:
		vs = fmt.Sprintf("%q", this.match)
	}
	return fmt.Sprintf("%s[%s=%s]", this.node.String(), this.path, vs)
}

func (this *SelectionNode) value(v reflect.Value, addMissing bool) (reflect.Value, error) {
	v = toValue(v, addMissing)
	if v.Kind() != reflect.Array && v.Kind() != reflect.Slice {
		return reflect.Value{}, fmt.Errorf("%s is no slice or array(%s) ", this.node.String(), v.Type())
	}
	index := -1
	for i := 0; i < v.Len(); i++ {
		v.Index(i)
		e := toValue(v.Index(i), true)

		match, err := this.path.Get(e)
		if err != nil {
			return reflect.Value{}, err
		}
		if match == this.match {
			index = i
		}
	}
	if index < 0 {
		if !addMissing || v.Kind() == reflect.Array {
			return reflect.Value{}, fmt.Errorf("no matching element found (%s=%s)", this.path, this.match)
		} else {
			e := reflect.New(v.Type().Elem()).Elem()
			new := toValue(e, true)
			err := this.path.Set(new, this.match)
			if err != nil {
				return reflect.Value{}, err
			}
			index = v.Len()
			v.Set(reflect.Append(v, e))
		}
	}
	return v.Index(index), nil
}
