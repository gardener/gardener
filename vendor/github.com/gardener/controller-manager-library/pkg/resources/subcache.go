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
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Owners func(sub Object) ClusterObjectKeySet

type SubObjectCache struct {
	lock    sync.Mutex
	byOwner map[ClusterObjectKey]ClusterObjectKeySet
	nested  map[ClusterObjectKey]*entry
	filters []OwnerFilter
	owners  Owners
}

func NewSubObjectCache(o Owners) *SubObjectCache {
	return &SubObjectCache{byOwner: map[ClusterObjectKey]ClusterObjectKeySet{}, nested: map[ClusterObjectKey]*entry{}, owners: o}
}

func (this *SubObjectCache) filterObject(obj Object) bool {
	return this.filterKey(obj.ClusterKey())
}

func (this *SubObjectCache) filterKey(key ClusterObjectKey) bool {
	if this.filters == nil {
		return true
	}
	for _, f := range this.filters {
		if f(key) {
			return true
		}
	}
	return false
}

func (this *SubObjectCache) AddOwnerFilter(filters ...OwnerFilter) *SubObjectCache {
	this.filters = append(this.filters, filters...)
	return this
}

func (this *SubObjectCache) Size() int {
	return len(this.byOwner)
}

func (this *SubObjectCache) SubObjectCount() int {
	return len(this.nested)
}

func (this *SubObjectCache) Setup(subObjects []Object) {
	this.lock.Lock()
	defer this.lock.Unlock()
	for _, s := range subObjects {
		for o := range this.owners(s) {
			this.add(o, s)
		}
	}
}

func (this *SubObjectCache) GetSubObject(key ClusterObjectKey) Object {
	this.lock.Lock()
	defer this.lock.Unlock()
	o := this.nested[key]
	if o == nil {
		return nil
	}
	return o.object
}

func (this *SubObjectCache) GetOwners(key ClusterObjectKey, kinds ...schema.GroupKind) ClusterObjectKeySet {
	o := this.GetSubObject(key)
	if o == nil {
		return ClusterObjectKeySet{}
	}
	return FilterKeysByGroupKinds(this.owners(o), kinds...)
}

func (this *SubObjectCache) GetAllOwners(kind ...schema.GroupKind) ClusterObjectKeySet {
	kinds := NewGroupKindSet(kind...)
	set := ClusterObjectKeySet{}

	this.lock.Lock()
	defer this.lock.Unlock()
	for k := range this.byOwner {
		if len(kinds) > 0 && !kinds.Contains(k.GroupKind()) {
			continue
		}
		set.Add(k)
	}
	return set
}

func (this *SubObjectCache) DeleteSubObject(key ClusterObjectKey) {
	this.lock.Lock()
	defer this.lock.Unlock()

	entry := this.nested[key]
	if entry != nil {
		for o := range this.owners(entry.object) {
			this.removeByKey(o, key)
		}
	}
}

func (this *SubObjectCache) DeleteOwner(key ClusterObjectKey) {
	this.lock.Lock()
	defer this.lock.Unlock()

	slaves := this.byOwner[key]
	if slaves != nil {
		for s := range slaves {
			this.removeByKey(key, s)
		}
		delete(this.byOwner, key)
	}
}

func (this *SubObjectCache) RenewSubObject(obj Object) bool {
	this.lock.Lock()
	defer this.lock.Unlock()
	return this.renewSubObject(obj)
}

func (this *SubObjectCache) UpdateSubObject(obj Object) error {
	this.lock.Lock()
	defer this.lock.Unlock()
	err := obj.Update()
	if err == nil {
		this.renewSubObject(obj)
	}
	return err
}

func (this *SubObjectCache) renewSubObject(obj Object) bool {
	key := obj.ClusterKey()
	entry := this.nested[key]
	newowners := this.owners(obj)
	if len(newowners) == 0 && entry == nil {
		return false
	}
	if entry != nil {
		add, del := newowners.DiffFrom(this.owners(entry.object))
		for e := range add {
			this.add(e, obj)
		}
		for e := range del {
			this.remove(e, obj)
		}
		entry.object = obj
		return len(add)+len(del) > 0
	}
	for e := range newowners {
		this.add(e, obj)
	}
	return true
}

func (this *SubObjectCache) Get(obj Object) []Object {
	return this.GetByKey(obj.ClusterKey())
}

func (this *SubObjectCache) GetByKey(key ClusterObjectKey) []Object {
	this.lock.Lock()
	defer this.lock.Unlock()
	keys := this.byOwner[key]
	result := []Object{}
	for k := range keys {
		result = append(result, this.nested[k].object)
	}
	return result
}

func (this *SubObjectCache) CreateSubObject(sub Object) error {
	this.lock.Lock()
	defer this.lock.Unlock()
	err := sub.Create()
	if err == nil {
		this.renewSubObject(sub)
	}
	return err
}

func (this *SubObjectCache) add(owner ClusterObjectKey, sub Object) {
	if !this.filterKey(owner) {
		return
	}
	key := sub.ClusterKey()
	set := this.byOwner[owner]
	if set == nil {
		set = ClusterObjectKeySet{}
		this.byOwner[owner] = set
	}
	e := this.nested[key]
	if !set.Contains(key) {
		set.Add(key)
		if e == nil {
			e = &entry{}
			this.nested[key] = e
		}
		e.count++
	}
	e.object = sub
}

func (this *SubObjectCache) remove(owner ClusterObjectKey, sub Object) {
	e := this.removeByKey(owner, sub.ClusterKey())
	if e != nil {
		e.object = sub
	}
}

func (this *SubObjectCache) removeByKey(owner ClusterObjectKey, sub ClusterObjectKey) *entry {
	set := this.byOwner[owner]
	if set != nil && set.Contains(sub) {
		set.Remove(sub)
		if len(set) == 0 {
			delete(this.byOwner, owner)
		}
		e := this.nested[sub]
		if e != nil {
			e.count--
			if e.count <= 0 {
				delete(this.nested, sub)
			}
		}
		return e
	}
	return nil
}
