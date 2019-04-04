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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type AbstractObject struct {
	ObjectData
	self I_Object
}

func NewAbstractObject(self I_Object, data ObjectData) AbstractObject {
	return AbstractObject{data, self}
}

func (this *AbstractObject) Data() ObjectData {
	return this.ObjectData
}

func (this *AbstractObject) ObjectName() ObjectName {
	return NewObjectName(this.GetNamespace(), this.GetName())
}

func (this *AbstractObject) Key() ObjectKey {
	return NewKey(this.GroupKind(), this.GetNamespace(), this.GetName())
}

func (this *AbstractObject) ClusterKey() ClusterObjectKey {
	return NewClusterKey(this.self.GetCluster().GetId(), this.GroupKind(), this.GetNamespace(), this.GetName())
}

func (this *AbstractObject) Description() string {
	return fmt.Sprintf("%s:%s", this.self.GetCluster().GetName(), this.Key())
}

func (this *AbstractObject) IsCoLocatedTo(o Object) bool {
	if o == nil {
		return true
	}
	return o.GetCluster() == this.self.GetCluster()
}

func (this *AbstractObject) Resources() Resources {
	return this.self.GetCluster().Resources()
}

func (this *AbstractObject) Event(eventtype, reason, message string) {
	this.self.GetCluster().Resources().Event(this.ObjectData, eventtype, reason, message)
}

func (this *AbstractObject) Eventf(eventtype, reason, messageFmt string, args ...interface{}) {
	this.self.GetCluster().Resources().Eventf(this.ObjectData, eventtype, reason, messageFmt, args...)
}

func (this *AbstractObject) PastEventf(timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {
	this.self.GetCluster().Resources().PastEventf(this.ObjectData, timestamp, eventtype, reason, messageFmt, args...)
}

func (this *AbstractObject) AnnotatedEventf(annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	this.self.GetCluster().Resources().AnnotatedEventf(this.ObjectData, annotations, eventtype, reason, messageFmt, args...)
}

func (this *AbstractObject) GroupKind() schema.GroupKind {
	return this.self.GetResource().GroupKind()
}

func (this *AbstractObject) GroupVersionKind() schema.GroupVersionKind {
	return this.self.GetResource().GroupVersionKind()
}
