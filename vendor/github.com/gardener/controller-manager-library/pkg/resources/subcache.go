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
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type OwnerDetector func(sub Object) ClusterObjectKeySet

type SubObjectCache struct {
	lock         sync.Mutex
	byOwner      map[ClusterObjectKey]ClusterObjectKeySet
	subFilters   []ObjectFilter
	subObjects   map[ClusterObjectKey]*entry
	ownerFilters []KeyFilter
	owners       OwnerDetector
}

func NewSubObjectCache(o OwnerDetector) *SubObjectCache {
	return &SubObjectCache{
		byOwner:    map[ClusterObjectKey]ClusterObjectKeySet{},
		subObjects: map[ClusterObjectKey]*entry{},
		owners:     o,
	}
}

func (this *SubObjectCache) filterOwner(obj Object) bool {
	return this.filterOwnerKey(obj.ClusterKey())
}

func (this *SubObjectCache) filterOwnerKey(key ClusterObjectKey) bool {
	if this.ownerFilters == nil {
		return true
	}
	for _, f := range this.ownerFilters {
		if f(key) {
			return true
		}
	}
	return false
}

func (this *SubObjectCache) filterSubObject(obj Object) bool {
	if this.subFilters == nil {
		return true
	}
	for _, f := range this.subFilters {
		if f(obj) {
			return true
		}
	}
	return false
}

func (this *SubObjectCache) AddOwnerFilter(filters ...KeyFilter) *SubObjectCache {
	this.ownerFilters = append(this.ownerFilters, filters...)
	return this
}

func (this *SubObjectCache) AddSubObjectFilter(filters ...ObjectFilter) *SubObjectCache {
	this.subFilters = append(this.subFilters, filters...)
	return this
}

func (this *SubObjectCache) Size() int {
	return len(this.byOwner)
}

func (this *SubObjectCache) SubObjectCount() int {
	return len(this.subObjects)
}

func (this *SubObjectCache) Setup(subObjects []Object) {
	this.lock.Lock()
	defer this.lock.Unlock()
	for _, s := range subObjects {
		if this.filterSubObject(s) {
			for o := range this.owners(s) {
				this.add(o, s)
			}
		}
	}
}

func (this *SubObjectCache) GetSubObject(key ClusterObjectKey) Object {
	this.lock.Lock()
	defer this.lock.Unlock()
	o := this.subObjects[key]
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

	entry := this.subObjects[key]
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
	if !this.filterSubObject(obj) {
		return false
	}
	this.lock.Lock()
	defer this.lock.Unlock()
	return this.renewSubObject(obj)
}

func (this *SubObjectCache) UpdateSubObject(obj Object) error {
	if !this.filterSubObject(obj) {
		return nil
	}
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
	entry := this.subObjects[key]
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

// Get is replaced by GetByOwner
// Deprecated: Please use GetByOwner
func (this *SubObjectCache) Get(obj Object) []Object {
	return this.GetByOwner(obj)
}
func (this *SubObjectCache) GetByOwner(obj Object) []Object {
	return this.GetByOwnerKey(obj.ClusterKey())
}

// GetByKey is replaced by GetByOwnerKey
// Deprecated: Please use GetByOwnerKey
func (this *SubObjectCache) GetByKey(key ClusterObjectKey) []Object {
	return this.GetByOwnerKey(key)
}
func (this *SubObjectCache) GetByOwnerKey(key ClusterObjectKey) []Object {
	this.lock.Lock()
	defer this.lock.Unlock()
	keys := this.byOwner[key]
	result := []Object{}
	for k := range keys {
		result = append(result, this.subObjects[k].object)
	}
	return result
}

func (this *SubObjectCache) CreateSubObject(sub Object) error {
	if !this.filterSubObject(sub) {
		return fmt.Errorf("sub object %s is rejected from sub object cache", sub.ClusterKey())
	}
	this.lock.Lock()
	defer this.lock.Unlock()
	err := sub.Create()
	if err == nil {
		this.renewSubObject(sub)
	}
	return err
}

func (this *SubObjectCache) add(owner ClusterObjectKey, sub Object) {
	if !this.filterOwnerKey(owner) {
		return
	}
	key := sub.ClusterKey()
	set := this.byOwner[owner]
	if set == nil {
		set = ClusterObjectKeySet{}
		this.byOwner[owner] = set
	}
	e := this.subObjects[key]
	if !set.Contains(key) {
		set.Add(key)
		if e == nil {
			e = &entry{}
			this.subObjects[key] = e
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
		e := this.subObjects[sub]
		if e != nil {
			e.count--
			if e.count <= 0 {
				delete(this.subObjects, sub)
			}
		}
		return e
	}
	return nil
}
