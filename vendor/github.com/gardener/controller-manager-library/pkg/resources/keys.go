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
// ObjectKey
////////////////////////////////////////////////////////////////////////////////

var _ GroupKindProvider = ObjectKey{}

func NewKey(groupKind schema.GroupKind, namespace, name string) ObjectKey {
	return ObjectKey{groupKind, NewObjectName(namespace, name)}
}

func (this ObjectKey) GroupKind() schema.GroupKind {
	return this.groupKind
}

func (this ObjectKey) Group() string {
	return this.groupKind.Group
}

func (this ObjectKey) Kind() string {
	return this.groupKind.Kind
}

func (this ObjectKey) Namespace() string {
	return this.name.Namespace()
}

func (this ObjectKey) ObjectName() ObjectName {
	return this.name
}

func (this ObjectKey) Name() string {
	return this.name.Name()
}

func (this ObjectKey) ForCluster(id string) ClusterObjectKey {
	return ClusterObjectKey{id, objectKey{this}}
}

func (this ObjectKey) String() string {
	return fmt.Sprintf("%s/%s/%s", this.groupKind.Group, this.groupKind.Kind, this.name)
}

func NewGroupKind(group, kind string) schema.GroupKind {
	if group == "core" {
		group = ""
	}
	return schema.GroupKind{Group: group, Kind: kind}
}

////////////////////////////////////////////////////////////////////////////////
// ClusterObjectKey
////////////////////////////////////////////////////////////////////////////////

func NewClusterKeyForObject(cluster string, key ObjectKey) ClusterObjectKey {
	return ClusterObjectKey{cluster, objectKey{key}}
}

func NewClusterKey(cluster string, groupKind schema.GroupKind, namespace, name string) ClusterObjectKey {
	return ClusterObjectKey{cluster, objectKey{ObjectKey{groupKind, NewObjectName(namespace, name)}}}
}

func (this ClusterObjectKey) String() string {
	return this.asString()
}

func (this ClusterObjectKey) asString() string {
	return fmt.Sprintf("%s:%s", this.cluster, this.objectKey.ObjectKey)
}

func (this ClusterObjectKey) Cluster() string {
	return this.cluster
}

func (this ClusterObjectKey) ObjectKey() ObjectKey {
	return this.objectKey.ObjectKey
}

func (this ClusterObjectKey) AsRefFor(clusterid string) string {
	if this.cluster == clusterid {
		return this.objectKey.String()
	}
	return this.asString()
}

func ParseClusterObjectKey(clusterid string, key string) (ClusterObjectKey, error) {
	id := clusterid
	i := strings.Index(key, ":")
	if i >= 0 {
		id = key[:i]
		key = key[i+1:]
	}
	comps := strings.Split(key, "/")
	switch len(comps) {
	case 4:
		return NewClusterKey(id, NewGroupKind(comps[0], comps[1]), comps[2], comps[3]), nil
	default:
		return ClusterObjectKey{}, fmt.Errorf("invalid cluster object key format")
	}
}

////////////////////////////////////////////////////////////////////////////////
// Cluster Object Key Set
////////////////////////////////////////////////////////////////////////////////

type ClusterObjectKeySet map[ClusterObjectKey]struct{}

func NewClusterObjectKeySet(a ...ClusterObjectKey) ClusterObjectKeySet {
	return ClusterObjectKeySet{}.Add(a...)
}

func NewClusterObjectKeySetByArray(a []ClusterObjectKey) ClusterObjectKeySet {
	s := ClusterObjectKeySet{}
	if a != nil {
		s.Add(a...)
	}
	return s
}

func NewClusterObjectKeSetBySets(sets ...ClusterObjectKeySet) ClusterObjectKeySet {
	s := ClusterObjectKeySet{}
	for _, set := range sets {
		for a := range set {
			s.Add(a)
		}
	}
	return s
}

func (this ClusterObjectKeySet) String() string {
	sep := ""
	data := "["
	for k := range this {
		data = fmt.Sprintf("%s%s'%s'", data, sep, k)
		sep = ", "
	}
	return data + "]"
}

func (this ClusterObjectKeySet) Contains(n ClusterObjectKey) bool {
	_, ok := this[n]
	return ok
}

func (this ClusterObjectKeySet) Remove(n ClusterObjectKey) ClusterObjectKeySet {
	delete(this, n)
	return this
}

func (this ClusterObjectKeySet) AddAll(n []ClusterObjectKey) ClusterObjectKeySet {
	return this.Add(n...)
}

func (this ClusterObjectKeySet) Add(n ...ClusterObjectKey) ClusterObjectKeySet {
	for _, p := range n {
		this[p] = struct{}{}
	}
	return this
}

func (this ClusterObjectKeySet) AddSet(sets ...ClusterObjectKeySet) ClusterObjectKeySet {
	for _, s := range sets {
		for e := range s {
			this.Add(e)
		}
	}
	return this
}

func (this ClusterObjectKeySet) Equals(set ClusterObjectKeySet) bool {
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

func (this ClusterObjectKeySet) DiffFrom(set ClusterObjectKeySet) (add, del ClusterObjectKeySet) {
	add = ClusterObjectKeySet{}
	del = ClusterObjectKeySet{}
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

func (this ClusterObjectKeySet) Copy() ClusterObjectKeySet {
	set := NewClusterObjectKeySet()
	for n := range this {
		set[n] = struct{}{}
	}
	return set
}

func (this ClusterObjectKeySet) AsArray() []ClusterObjectKey {
	a := []ClusterObjectKey{}
	for n := range this {
		a = append(a, n)
	}
	return a
}

////////////////////////////////////////////////////////////////////////////////
// Group Kind Set
////////////////////////////////////////////////////////////////////////////////

type GroupKindSet map[schema.GroupKind]struct{}

func NewGroupKindSet(a ...schema.GroupKind) GroupKindSet {
	return GroupKindSet{}.Add(a...)
}

func NewGroupKindSetByArray(a []schema.GroupKind) GroupKindSet {
	s := GroupKindSet{}
	if a != nil {
		s.Add(a...)
	}
	return s
}

func NewsGroupKindSetBySets(sets ...GroupKindSet) GroupKindSet {
	s := GroupKindSet{}
	for _, set := range sets {
		for a := range set {
			s.Add(a)
		}
	}
	return s
}

func (this GroupKindSet) String() string {
	sep := ""
	data := "["
	for k := range this {
		data = fmt.Sprintf("%s%s'%s'", data, sep, k)
		sep = ", "
	}
	return data + "]"
}

func (this GroupKindSet) Contains(n schema.GroupKind) bool {
	_, ok := this[n]
	return ok
}

func (this GroupKindSet) Remove(n schema.GroupKind) GroupKindSet {
	delete(this, n)
	return this
}

func (this GroupKindSet) AddAll(n []schema.GroupKind) GroupKindSet {
	return this.Add(n...)
}

func (this GroupKindSet) Add(n ...schema.GroupKind) GroupKindSet {
	for _, p := range n {
		this[p] = struct{}{}
	}
	return this
}

func (this GroupKindSet) AddSet(sets ...GroupKindSet) GroupKindSet {
	for _, s := range sets {
		for e := range s {
			this.Add(e)
		}
	}
	return this
}

func (this GroupKindSet) Equals(set GroupKindSet) bool {
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

func (this GroupKindSet) DiffFrom(set GroupKindSet) (add, del GroupKindSet) {
	add = GroupKindSet{}
	del = GroupKindSet{}
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

func (this GroupKindSet) Copy() GroupKindSet {
	set := NewGroupKindSet()
	for n := range this {
		set[n] = struct{}{}
	}
	return set
}

func (this GroupKindSet) AsArray() []schema.GroupKind {
	a := []schema.GroupKind{}
	for n := range this {
		a = append(a, n)
	}
	return a
}
