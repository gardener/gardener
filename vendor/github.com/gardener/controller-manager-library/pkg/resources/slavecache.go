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
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type OwnerFilter func(key ClusterObjectKey) bool

func NewGroupKindFilter(gk schema.GroupKind) OwnerFilter {
	return func(key ClusterObjectKey) bool {
		return key.GroupKind() == gk
	}
}

type entry struct {
	object Object
	count  int
}

type SlaveCache struct {
	cache SubObjectCache
}

func NewSlaveCache() *SlaveCache {
	return &SlaveCache{*NewSubObjectCache(func(o Object) ClusterObjectKeySet { return o.GetOwners() })}
}

func (this *SlaveCache) AddOwnerFilter(filters ...OwnerFilter) *SlaveCache {
	this.cache.AddOwnerFilter(filters...)
	return this
}

func (this *SlaveCache) Size() int {
	return this.cache.Size()
}

func (this *SlaveCache) SlaveCount() int {
	return this.cache.SubObjectCount()
}

func (this *SlaveCache) Setup(slaves []Object) {
	this.cache.Setup(slaves)
}

func (this *SlaveCache) GetSlave(key ClusterObjectKey) Object {
	return this.cache.GetSubObject(key)
}

func (this *SlaveCache) GetOwners(kinds ...schema.GroupKind) ClusterObjectKeySet {
	return this.cache.GetAllOwners(kinds...)
}

func (this *SlaveCache) GetOwnersFor(key ClusterObjectKey, kinds ...schema.GroupKind) ClusterObjectKeySet {
	o := this.GetSlave(key)
	if o == nil {
		return ClusterObjectKeySet{}
	}
	return o.GetOwners(kinds...)
}

func (this *SlaveCache) DeleteSlave(key ClusterObjectKey) {
	this.cache.DeleteSubObject(key)
}

func (this *SlaveCache) DeleteOwner(key ClusterObjectKey) {
	this.cache.DeleteOwner(key)
}

func (this *SlaveCache) RenewSlaveObject(obj Object) bool {
	return this.cache.RenewSubObject(obj)
}

func (this *SlaveCache) UpdateSlave(obj Object) error {
	return this.cache.UpdateSubObject(obj)
}

func (this *SlaveCache) Get(obj Object) []Object {
	return this.cache.Get(obj)
}

func (this *SlaveCache) GetByKey(key ClusterObjectKey) []Object {
	return this.cache.GetByKey(key)
}

func (this *SlaveCache) AddSlave(obj Object, slave Object) error {
	if slave.AddOwner(obj) {
		return this.cache.UpdateSubObject(slave)
	}
	return nil
}

func (this *SlaveCache) CreateSlave(obj Object, slave Object) error {
	slave.AddOwner(obj)
	return this.cache.CreateSubObject(slave)
}

func (this *SlaveCache) Remove(obj Object, slave Object) bool {
	mod := slave.RemoveOwner(obj)
	if mod {
		this.cache.UpdateSubObject(slave)
	}
	return mod
}
