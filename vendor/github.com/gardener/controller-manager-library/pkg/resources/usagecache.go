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

type UsedExtractor func(sub Object) ClusterObjectKeySet

type UsageCache struct {
	lock    sync.Mutex
	byOwner map[ClusterObjectKey]ClusterObjectKeySet
	byUsed  map[ClusterObjectKey]ClusterObjectKeySet
	filters []OwnerFilter
	used    UsedExtractor
}

func NewUsageCache(u UsedExtractor) *UsageCache {
	return &UsageCache{byOwner: map[ClusterObjectKey]ClusterObjectKeySet{}, byUsed: map[ClusterObjectKey]ClusterObjectKeySet{}, used: u}
}

func (this *UsageCache) filterKey(key ClusterObjectKey) bool {
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

func (this *UsageCache) AddOwnerFilter(filters ...OwnerFilter) *UsageCache {
	this.filters = append(this.filters, filters...)
	return this
}

func (this *UsageCache) Size() int {
	return len(this.byOwner)
}

func (this *UsageCache) UsedCount() int {
	return len(this.byUsed)
}

func (this *UsageCache) Setup(owners []Object) {
	this.lock.Lock()
	defer this.lock.Unlock()
	for _, s := range owners {
		for o := range this.used(s) {
			this.add(s.ClusterKey(), o)
		}
	}
}

func (this *UsageCache) GetUsages(key ClusterObjectKey) ClusterObjectKeySet {
	this.lock.Lock()
	defer this.lock.Unlock()
	o := this.byOwner[key]
	if o == nil {
		return ClusterObjectKeySet{}
	}
	return o.Copy()
}

func (this *UsageCache) GetOwners() ClusterObjectKeySet {
	set := ClusterObjectKeySet{}

	for k := range this.byOwner {
		set.Add(k)
	}
	return set
}

func (this *UsageCache) GetUsed() ClusterObjectKeySet {
	set := ClusterObjectKeySet{}

	for k := range this.byUsed {
		set.Add(k)
	}
	return set
}

func (this *UsageCache) GetOwnersFor(key ClusterObjectKey, kinds ...schema.GroupKind) ClusterObjectKeySet {
	this.lock.Lock()
	defer this.lock.Unlock()

	return FilterKeysByGroupKinds(this.byUsed[key], kinds...)
}

func (this *UsageCache) DeleteOwner(key ClusterObjectKey) {
	this.lock.Lock()
	defer this.lock.Unlock()

	used := this.byOwner[key]
	if len(used) > 0 {
		for s := range used {
			this.removeUsage(key, s)
		}
	}
	delete(this.byOwner, key)
}

func (this *UsageCache) RenewOwner(obj Object) bool {
	this.lock.Lock()
	defer this.lock.Unlock()
	return this.renewOwner(obj)
}

func (this *UsageCache) renewOwner(obj Object) bool {
	key := obj.ClusterKey()
	oldused := this.byOwner[key]
	ńewused := this.used(obj)
	if len(ńewused) == 0 && len(oldused) == 0 {
		return false
	}
	if len(oldused) > 0 {
		add, del := ńewused.DiffFrom(oldused)
		for e := range add {
			this.add(key, e)
		}
		for e := range del {
			this.remove(key, e)
		}
		return len(add)+len(del) > 0
	}
	for e := range ńewused {
		this.add(key, e)
	}
	return true
}

func (this *UsageCache) add(owner ClusterObjectKey, key ClusterObjectKey) {
	if !this.filterKey(owner) {
		return
	}
	// add used to owner
	set := this.byOwner[owner]
	if set == nil {
		set = ClusterObjectKeySet{}
		this.byOwner[owner] = set
	}
	set.Add(key)

	// add owner to used
	set = this.byUsed[key]
	if set == nil {
		set = ClusterObjectKeySet{}
		this.byUsed[key] = set
	}
	set.Add(owner)
}

func (this *UsageCache) remove(owner ClusterObjectKey, key ClusterObjectKey) {
	// remove used to owner
	set := this.byOwner[owner]
	if set != nil {
		set.Remove(key)
	}

	this.removeUsage(owner, key)
}

func (this *UsageCache) removeUsage(owner ClusterObjectKey, key ClusterObjectKey) {
	set := this.byUsed[key]
	if set != nil {
		set.Remove(owner)
	}
}
