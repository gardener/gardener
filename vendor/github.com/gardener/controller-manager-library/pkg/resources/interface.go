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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

type GroupKindProvider interface {
	GroupKind() schema.GroupKind
}

// objectKey is just used to allow a method ObjectKey for ClusterObjectKey
type objectKey struct {
	ObjectKey
}

type ClusterObjectKey struct {
	cluster string
	objectKey
}

// ObjectKey used for worker queues.
type ObjectKey struct {
	groupKind schema.GroupKind
	name      ObjectName
}

type ResourcesSource interface {
	Resources() Resources
}

type ClusterSource interface {
	GetCluster() Cluster
}

type Cluster interface {
	ResourcesSource
	ClusterSource

	GetName() string
	GetId() string
	Config() restclient.Config

	GetAttr(key interface{}) interface{}
	SetAttr(key, value interface{})
}

/////////////////////////////////////////////////////////////////////////////////

type EventRecorder interface {
	Event(eventtype, reason, message string)

	// Eventf is just like Event, but with Sprintf for the message field.
	Eventf(eventtype, reason, messageFmt string, args ...interface{})

	// PastEventf is just like Eventf, but with an option to specify the event's 'timestamp' field.
	PastEventf(timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{})

	// AnnotatedEventf is just like eventf, but with annotations attached
	AnnotatedEventf(annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{})
}

type ResourceEventHandlerFuncs struct {
	AddFunc    func(obj Object)
	UpdateFunc func(oldObj, newObj Object)
	DeleteFunc func(obj Object)
}

type Modifier func(ObjectData) (bool, error)

type Object interface {
	metav1.Object
	GroupKindProvider
	//runtime.ObjectData
	EventRecorder
	ResourcesSource
	ClusterSource

	GroupVersionKind() schema.GroupVersionKind
	ObjectName() ObjectName
	Data() ObjectData
	DeepCopy() Object
	Key() ObjectKey
	ClusterKey() ClusterObjectKey
	IsCoLocatedTo(o Object) bool

	GetResource() Interface

	IsA(spec interface{}) bool
	Create() error
	CreateOrUpdate() error
	Delete() error
	Update() error
	UpdateStatus() error
	Modify(modifier Modifier) (bool, error)
	ModifyStatus(modifier Modifier) (bool, error)
	CreateOrModify(modifier Modifier) (bool, error)
	UpdateFromCache() error

	Description() string
	HasFinalizer(key string) bool
	SetFinalizer(key string) error
	RemoveFinalizer(key string) error

	GetLabel(name string) string

	IsDeleting() bool

	GetOwnerReference() *metav1.OwnerReference
	GetOwners(kinds ...schema.GroupKind) ClusterObjectKeySet
	AddOwner(Object) bool
	RemoveOwner(Object) bool
}

type ObjectMatcher func(Object) bool

type ObjectNameProvider interface {
	Namespace() string
	Name() string
}

type ObjectName interface {
	Name() string
	Namespace() string
	String() string

	ForGroupKind(gk schema.GroupKind) ObjectKey
}

type ObjectDataName interface {
	GetName() string
	GetNamespace() string
}

type ObjectData interface {
	metav1.Object
	runtime.Object
}

type Interface interface {
	GroupKindProvider
	ClusterSource
	ResourcesSource

	Name() string
	Namespaced() bool
	GroupVersionKind() schema.GroupVersionKind
	Info() *Info
	ResourceContext() ResourceContext
	AddEventHandler(eventHandlers ResourceEventHandlerFuncs) error
	AddRawEventHandler(handlers cache.ResourceEventHandlerFuncs) error

	Wrap(ObjectData) (Object, error)
	New(ObjectName) Object

	GetInto(ObjectName, ObjectData) (Object, error)

	GetCached(interface{}) (Object, error)
	Get_(obj interface{}) (Object, error)
	ListCached(selector labels.Selector) ([]Object, error)
	List(opts metav1.ListOptions) (ret []Object, err error)
	Create(ObjectData) (Object, error)
	CreateOrUpdate(obj ObjectData) (Object, error)
	Update(ObjectData) (Object, error)
	Delete(ObjectData) error
	DeleteByName(ObjectDataName) error

	Namespace(name string) Namespaced

	IsUnstructured() bool
}

type Namespaced interface {
	ListCached(selector labels.Selector) ([]Object, error)
	List(opts metav1.ListOptions) (ret []Object, err error)
	GetCached(name string) (Object, error)
	Get(name string) (Object, error)
}

type Resources interface {
	ResourcesSource
	record.EventRecorder

	Get(interface{}) (Interface, error)
	GetByExample(obj runtime.Object) (Interface, error)
	GetByGK(gk schema.GroupKind) (Interface, error)
	GetByGVK(gvk schema.GroupVersionKind) (Interface, error)

	GetUnstructuredByGK(gk schema.GroupKind) (Interface, error)
	GetUnstructuredByGVK(gvk schema.GroupVersionKind) (Interface, error)

	Wrap(obj ObjectData) (Object, error)

	GetObjectInto(ObjectName, ObjectData) (Object, error)

	GetObject(spec interface{}) (Object, error)
	GetCachedObject(spec interface{}) (Object, error)

	CreateObject(ObjectData) (Object, error)
	CreateOrUpdateObject(obj ObjectData) (Object, error)

	DeleteObject(obj ObjectData) error
}
