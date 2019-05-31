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
	"github.com/gardener/controller-manager-library/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
)

var cluster_key reflect.Type

func init() {
	cluster_key, _ = utils.TypeKey((*Cluster)(nil))
}

// _object is the standard implementation of the Object interface
// it uses the AbstractObject as base to provide standard implementations
// based on the internal object interface. (see _i_object)
type _object struct {
	AbstractObject
	cluster  Cluster
	resource Internal
}

var _ Object = &_object{}

func NewObject(data ObjectData, cluster Cluster, resource Internal) Object {
	o := &_object{AbstractObject{}, cluster, resource}
	o.AbstractObject = NewAbstractObject(&_i_object{o}, data)
	return o
}

func (this *_object) DeepCopy() Object {
	r := &_object{AbstractObject{}, this.cluster, this.resource}
	r.AbstractObject = NewAbstractObject(&_i_object{r}, this.ObjectData.DeepCopyObject().(ObjectData))
	return r
}

/////////////////////////////////////////////////////////////////////////////////

func (this *_object) GetCluster() Cluster {
	return this.cluster
}

func (this *_object) GetResource() Interface {
	return this.resource
}

func (this *_object) IsA(spec interface{}) bool {
	switch s := spec.(type) {
	case GroupKindProvider:
		return s.GroupKind() == this.GroupKind()
	case runtime.Object:
		t := reflect.TypeOf(s)
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		return t == this.resource.I_objectType()
	case schema.GroupVersionKind:
		return s == this.resource.GroupVersionKind()
	case *schema.GroupVersionKind:
		return *s == this.resource.GroupVersionKind()
	case schema.GroupKind:
		return s == this.GroupKind()
	case *schema.GroupKind:
		return *s == this.GroupKind()
	default:
		return false
	}
}
