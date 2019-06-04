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
	"reflect"

	"github.com/gardener/controller-manager-library/pkg/kutil"
	"github.com/gardener/controller-manager-library/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

const ATTR_EVENTSOURCE = "event-source"

type _resources struct {
	ctx                        *resourceContext
	informers                  *sharedInformerFactory
	handlersByObjType          map[reflect.Type]Interface
	handlersByGroupKind        map[schema.GroupKind]Interface
	handlersByGroupVersionKind map[schema.GroupVersionKind]Interface

	unstructuredHandlersByGroupKind        map[schema.GroupKind]Interface
	unstructuredHandlersByGroupVersionKind map[schema.GroupVersionKind]Interface

	record.EventRecorder
}

var _ Resources = &_resources{}

func newResources(c *resourceContext, source string) *_resources {
	res := _resources{}
	res.ctx = c
	res.informers = c.sharedInformerFactory
	res.handlersByObjType = map[reflect.Type]Interface{}
	res.handlersByGroupKind = map[schema.GroupKind]Interface{}
	res.handlersByGroupVersionKind = map[schema.GroupVersionKind]Interface{}

	res.unstructuredHandlersByGroupKind = map[schema.GroupKind]Interface{}
	res.unstructuredHandlersByGroupVersionKind = map[schema.GroupVersionKind]Interface{}

	client, _ := c.GetClient(schema.GroupVersion{"", "v1"})

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logger.Debugf)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: typedcorev1.New(client).Events("")})
	res.EventRecorder = eventBroadcaster.NewRecorder(c.scheme, corev1.EventSource{Component: source})

	return &res
}

func (this *_resources) Resources() Resources {
	return this
}

func (this *_resources) Get(spec interface{}) (Interface, error) {
	switch o := spec.(type) {
	case GroupKindProvider:
		return this.GetByGK(o.GroupKind())
	case runtime.Object:
		return this.GetByExample(o)
	case schema.GroupVersionKind:
		return this.GetByGVK(o)
	case *schema.GroupVersionKind:
		return this.GetByGVK(*o)
	case schema.GroupKind:
		return this.GetByGK(o)
	case *schema.GroupKind:
		return this.GetByGK(*o)

	case ObjectKey:
		return this.GetByGK(o.GroupKind())
	case *ObjectKey:
		return this.GetByGK(o.GroupKind())

	case ClusterObjectKey:
		return this.GetByGK(o.GroupKind())
	case *ClusterObjectKey:
		return this.GetByGK(o.GroupKind())

	default:
		return nil, fmt.Errorf("invalid spec type %T", spec)
	}
}

func (this *_resources) CreateObject(obj ObjectData) (Object, error) {
	r, err := this.GetByExample(obj)
	if err != nil {
		return nil, err
	}
	return r.Create(obj)
}
func (this *_resources) CreateOrUpdateObject(obj ObjectData) (Object, error) {
	r, err := this.GetByExample(obj)
	if err != nil {
		return nil, err
	}
	return r.CreateOrUpdate(obj)
}

func (this *_resources) DeleteObject(obj ObjectData) error {
	r, err := this.GetByExample(obj)
	if err != nil {
		return err
	}
	return r.Delete(obj)
}

func (this *_resources) GetByExample(obj runtime.Object) (Interface, error) {

	t := reflect.TypeOf(obj)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	lock.Lock()
	defer lock.Unlock()
	if handler, ok := this.handlersByObjType[t]; ok {
		return handler, nil
	}

	gvk, err := this.ctx.GetGVK(obj)
	if err != nil {
		return nil, err
	}

	info, err := this.ctx.Get(gvk)
	if err != nil {
		return nil, err
	}
	return this.newResource(gvk, t, info)

}

func (this *_resources) GetByGK(gk schema.GroupKind) (Interface, error) {
	lock.Lock()
	defer lock.Unlock()

	if handler, ok := this.handlersByGroupKind[gk]; ok {
		return handler, nil
	}

	info, err := this.informers.context.GetPreferred(gk)
	if err != nil {
		return nil, err
	}
	if handler, ok := this.handlersByGroupVersionKind[info.GroupVersionKind()]; ok {
		this.handlersByGroupKind[gk] = handler
		return handler, nil
	}

	h, err := this.getResource(info)
	if err != nil {
		return nil, err
	}
	this.handlersByGroupKind[gk] = h
	this.handlersByGroupVersionKind[info.GroupVersionKind()] = h
	return h, nil
}

func (this *_resources) GetByGVK(gvk schema.GroupVersionKind) (Interface, error) {
	lock.Lock()
	defer lock.Unlock()

	if handler, ok := this.handlersByGroupVersionKind[gvk]; ok {
		return handler, nil
	}

	info, err := this.ctx.Get(gvk)
	if err != nil {
		return nil, err
	}

	h, err := this.getResource(info)
	if err != nil {
		return nil, err
	}
	this.handlersByGroupVersionKind[gvk] = h
	return h, nil
}

func (this *_resources) getResource(info *Info) (Interface, error) {
	gvk := info.GroupVersionKind()
	informerType := this.ctx.KnownTypes(gvk.GroupVersion())[gvk.Kind]
	if informerType == nil {
		return nil, fmt.Errorf("%s unknown", gvk)
	}

	return this.newResource(gvk, informerType, info)
}

func (this *_resources) GetUnstructuredByGK(gk schema.GroupKind) (Interface, error) {
	lock.Lock()
	defer lock.Unlock()

	if handler, ok := this.unstructuredHandlersByGroupKind[gk]; ok {
		return handler, nil
	}

	info, err := this.informers.context.GetPreferred(gk)
	if err != nil {
		return nil, err
	}
	if handler, ok := this.unstructuredHandlersByGroupVersionKind[info.GroupVersionKind()]; ok {
		this.unstructuredHandlersByGroupKind[gk] = handler
		return handler, nil
	}

	h, err := this.getUnstructuredResource(info)
	if err != nil {
		return nil, err
	}
	this.unstructuredHandlersByGroupKind[gk] = h
	this.unstructuredHandlersByGroupVersionKind[info.GroupVersionKind()] = h
	return h, nil
}

func (this *_resources) GetUnstructuredByGVK(gvk schema.GroupVersionKind) (Interface, error) {
	lock.Lock()
	defer lock.Unlock()

	if handler, ok := this.unstructuredHandlersByGroupVersionKind[gvk]; ok {
		return handler, nil
	}

	info, err := this.ctx.Get(gvk)
	if err != nil {
		return nil, err
	}

	h, err := this.getUnstructuredResource(info)
	if err != nil {
		return nil, err
	}
	this.unstructuredHandlersByGroupVersionKind[gvk] = h
	return h, err
}

func (this *_resources) getUnstructuredResource(info *Info) (Interface, error) {
	gvk := info.GroupVersionKind()
	return this.newResource(gvk, nil, info)
}

func (this *_resources) Wrap(obj ObjectData) (Object, error) {
	h, err := this.GetByExample(obj)
	if err != nil {
		return nil, err
	}

	return h.Wrap(obj)
}

func (this *_resources) GetObject(spec interface{}) (Object, error) {
	h, err := this.Get(spec)
	if err != nil {
		return nil, err
	}

	return h.Get_(spec)
}

func (this *_resources) GetObjectInto(name ObjectName, obj ObjectData) (Object, error) {
	h, err := this.GetByExample(obj)
	if err != nil {
		return nil, err
	}

	return h.GetInto(name, obj)
}

func (this *_resources) GetCachedObject(spec interface{}) (Object, error) {
	h, err := this.Get(spec)
	if err != nil {
		return nil, err
	}

	return h.GetCached(spec)
}

func (r *_resources) newResource(gvk schema.GroupVersionKind, otype reflect.Type, info *Info) (Interface, error) {

	client, err := r.ctx.GetClient(gvk.GroupVersion())
	if err != nil {
		return nil, err
	}

	if otype == nil {
		otype = unstructuredType
	}
	ltype := kutil.DetermineListType(r.ctx.scheme, gvk.GroupVersion(), otype)
	if ltype == nil {
		return nil, fmt.Errorf("cannot determine list type for %s", otype)
	}

	handler := newResource(r.ctx, otype, ltype, info, client)
	return handler, nil
}
