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

package resources

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////
// Object Name
////////////////////////////////////////////////////////////////////////////////

type objectName struct {
	namespace string
	name      string
}

func NewObjectNameFor(p ObjectNameProvider) ObjectName {
	if p == nil {
		return nil
	}
	return NewObjectName(p.Namespace(), p.Name())
}

func NewObjectName(names ...string) ObjectName {
	switch len(names) {
	case 1:
		return objectName{"", names[0]}
	case 2:
		return objectName{names[0], names[1]}
	default:
		panic(fmt.Errorf("objectname has one or two arguments (got %d)", len(names)))
	}
}

func (this objectName) Namespace() string {
	return this.namespace
}

func (this objectName) Name() string {
	return this.name
}

func (this objectName) ForGroupKind(gk schema.GroupKind) ObjectKey {
	return NewKey(gk, this.namespace, this.name)
}

func (this objectName) String() string {
	return fmt.Sprintf("%s/%s", this.namespace, this.name)
}

func ParseObjectName(name string) (ObjectName, error) {
	comps := strings.Split(name, "/")
	switch len(comps) {
	case 0:
		return nil, nil
	case 1, 2:
		return NewObjectName(comps...), nil
	default:
		return nil, fmt.Errorf("illegal object anme %q", name)
	}
}

////////////////////////////////////////////////////////////////////////////////
// Object Name Set
////////////////////////////////////////////////////////////////////////////////

type ObjectNameSet map[ObjectName]struct{}

func NewObjectNameSet(a ...ObjectName) ObjectNameSet {
	return ObjectNameSet{}.Add(a...)
}

func NewObjectNameSetByArray(a []ObjectName) ObjectNameSet {
	s := ObjectNameSet{}
	if a != nil {
		s.Add(a...)
	}
	return s
}

func NewObjectNameSetBySets(sets ...ObjectNameSet) ObjectNameSet {
	s := ObjectNameSet{}
	for _, set := range sets {
		for a := range set {
			s.Add(a)
		}
	}
	return s
}

func (this ObjectNameSet) String() string {
	sep := ""
	data := "["
	for k := range this {
		data = fmt.Sprintf("%s%s'%s'", data, sep, k)
		sep = ", "
	}
	return data + "]"
}

func (this ObjectNameSet) Contains(n ObjectName) bool {
	_, ok := this[n]
	return ok
}

func (this ObjectNameSet) Remove(n ObjectName) ObjectNameSet {
	delete(this, n)
	return this
}

func (this ObjectNameSet) AddAll(n []ObjectName) ObjectNameSet {
	return this.Add(n...)
}

func (this ObjectNameSet) Add(n ...ObjectName) ObjectNameSet {
	for _, p := range n {
		this[p] = struct{}{}
	}
	return this
}

func (this ObjectNameSet) AddSet(sets ...ObjectNameSet) ObjectNameSet {
	for _, s := range sets {
		for e := range s {
			this.Add(e)
		}
	}
	return this
}

func (this ObjectNameSet) AddAllSplitted(n string) (ObjectNameSet, error) {
	for _, p := range strings.Split(n, ",") {
		o, err := ParseObjectName(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		this.Add(o)
	}
	return this, nil
}

func (this ObjectNameSet) Equals(set ObjectNameSet) bool {
	for n := range set {
		if !this.Contains(n) {
			return false
		}
	}
	for n := range this {
		if !set.Contains(n) {
			return false
		}
	}
	return true
}

func (this ObjectNameSet) DiffFrom(set ObjectNameSet) (add, del ObjectNameSet) {
	add = ObjectNameSet{}
	del = ObjectNameSet{}
	for n := range set {
		if !this.Contains(n) {
			add.Add(n)
		}
	}
	for n := range this {
		if !set.Contains(n) {
			del.Add(n)
		}
	}
	return
}

func (this ObjectNameSet) Copy() ObjectNameSet {
	set := NewObjectNameSet()
	for n := range this {
		set[n] = struct{}{}
	}
	return set
}

func (this ObjectNameSet) AsArray() []ObjectName {
	a := []ObjectName{}
	for n := range this {
		a = append(a, n)
	}
	return a
}
