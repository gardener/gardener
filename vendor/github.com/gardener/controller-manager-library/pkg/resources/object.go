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
	"github.com/gardener/controller-manager-library/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
)

var cluster_key reflect.Type

func init() {
	cluster_key, _ = utils.TypeKey((*Cluster)(nil))
}

type _object struct {
	ObjectData
	cluster  Cluster
	resource Internal
}

var _ Object = &_object{}

/////////////////////////////////////////////////////////////////////////////////

func (this *_object) Resources() Resources {
	return this.cluster.Resources()
}

func (this *_object) GetCluster() Cluster {
	return this.cluster
}

func (this *_object) GetObject() ObjectData {
	return this.ObjectData
}

func (this *_object) GetResource() Interface {
	return this.resource
}

func (this *_object) Event(eventtype, reason, message string) {
	this.cluster.Resources().Event(this.ObjectData, eventtype, reason, message)
}

func (this *_object) Eventf(eventtype, reason, messageFmt string, args ...interface{}) {
	this.cluster.Resources().Eventf(this.ObjectData, eventtype, reason, messageFmt, args...)
}

func (this *_object) PastEventf(timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {
	this.cluster.Resources().PastEventf(this.ObjectData, timestamp, eventtype, reason, messageFmt, args...)
}

func (this *_object) AnnotatedEventf(annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	this.cluster.Resources().AnnotatedEventf(this.ObjectData, annotations, eventtype, reason, messageFmt, args...)
}

func (this *_object) Data() ObjectData {
	return this.ObjectData
}

func (this *_object) ObjectName() ObjectName {
	return NewObjectName(this.GetNamespace(), this.GetName())
}

func (this *_object) Key() ObjectKey {
	return NewKey(this.GroupKind(), this.GetNamespace(), this.GetName())
}

func (this *_object) ClusterKey() ClusterObjectKey {
	return NewClusterKey(this.cluster.GetId(), this.GroupKind(), this.GetNamespace(), this.GetName())
}

func (this *_object) GroupKind() schema.GroupKind {
	return this.resource.GroupKind()
}

func (this *_object) GroupVersionKind() schema.GroupVersionKind {
	return this.resource.GroupVersionKind()
}

func (this *_object) Description() string {
	return fmt.Sprintf("%s:%s", this.GetCluster().GetName(), this.Key())
}

func (this *_object) IsCoLocatedTo(o Object) bool {
	if o == nil {
		return true
	}
	return o.GetCluster() == this.GetCluster()
}

func (this *_object) Resource() Interface {
	return this.resource
}

func (this *_object) DeepCopy() Object {
	r := &_object{this.ObjectData.DeepCopyObject().(ObjectData), this.cluster, this.resource}
	return r
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
		return t == this.resource.objectType()
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
