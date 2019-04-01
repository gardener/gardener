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

package utils

import (
	"fmt"
	"strings"
)

type StringSet map[string]struct{}

func NewStringSet(a ...string) StringSet {
	return StringSet{}.Add(a...)
}

func NewStringSetByArray(a []string) StringSet {
	s := StringSet{}
	if a != nil {
		s.Add(a...)
	}
	return s
}

func NewStringSetBySets(sets ...StringSet) StringSet {
	s := StringSet{}
	for _, set := range sets {
		for a := range set {
			s.Add(a)
		}
	}
	return s
}

func (this StringSet) String() string {
	sep := ""
	data := "["
	for k := range this {
		data = fmt.Sprintf("%s%s'%s'", data, sep, k)
		sep = ", "
	}
	return data + "]"
}

func (this StringSet) Clear() {
	for n := range this {
		delete(this, n)
	}
}

func (this StringSet) Contains(n string) bool {
	_, ok := this[n]
	return ok
}

func (this StringSet) Remove(n string) StringSet {
	delete(this, n)
	return this
}

func (this StringSet) AddAll(n []string) StringSet {
	return this.Add(n...)
}

func (this StringSet) Add(n ...string) StringSet {
	for _, p := range n {
		this[p] = struct{}{}
	}
	return this
}

func (this StringSet) AddSet(sets ...StringSet) StringSet {
	for _, s := range sets {
		for e := range s {
			this.Add(e)
		}
	}
	return this
}

func (this StringSet) AddAllSplitted(n string) StringSet {
	for _, p := range strings.Split(n, ",") {
		this.Add(strings.ToLower(strings.TrimSpace(p)))
	}
	return this
}

func (this StringSet) Equals(set StringSet) bool {
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

func (this StringSet) DiffFrom(set StringSet) (add, del StringSet) {
	add = StringSet{}
	del = StringSet{}
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

func (this StringSet) Copy() StringSet {
	set := NewStringSet()
	for n := range this {
		set[n] = struct{}{}
	}
	return set
}

func (this StringSet) AsArray() []string {
	a := []string{}
	for n := range this {
		a = append(a, n)
	}
	return a
}
