// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

var _ handler.EventHandler = (*EnqueueRequestsFromMapFunc)(nil)

// EnqueueRequestsFromMapFunc enqueues Requests by running a transformation function that outputs a collection
// of reconcile.Requests on each Event.  The reconcile.Requests may be for an arbitrary set of objects
// defined by some user specified transformation of the source Event.  (e.g. trigger Reconciler for a set of objects
// in response to a cluster resize event caused by adding or deleting a Node)
//
// EnqueueRequestsFromMapFunc is frequently used to fan-out updates from one object to one or more other
// objects of a differing type.
type EnqueueRequestsFromMapFunc struct {
	// Mapper transforms the argument into a slice of keys to be reconciled
	ToRequests Mapper
}

// InjectFunc implements Injector.
func (e *EnqueueRequestsFromMapFunc) InjectFunc(f inject.Func) error {
	return f(e.ToRequests)
}

// Create implements EventHandler
func (e *EnqueueRequestsFromMapFunc) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	enqueueRequests(q, e.ToRequests.MapCreate(MapCreateObject{Meta: evt.Meta, Object: evt.Object}))
}

// Update implements EventHandler
func (e *EnqueueRequestsFromMapFunc) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	enqueueRequests(q, e.ToRequests.MapUpdate(MapUpdateObject{
		MetaOld: evt.MetaOld, ObjectOld: evt.ObjectOld, MetaNew: evt.MetaNew, ObjectNew: evt.ObjectNew,
	}))
}

// Delete implements EventHandler
func (e *EnqueueRequestsFromMapFunc) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	enqueueRequests(q, e.ToRequests.MapDelete(MapDeleteObject{Meta: evt.Meta, Object: evt.Object}))
}

// Generic implements EventHandler
func (e *EnqueueRequestsFromMapFunc) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	enqueueRequests(q, e.ToRequests.MapGeneric(MapGenericObject{Meta: evt.Meta, Object: evt.Object}))
}

func enqueueRequests(q workqueue.RateLimitingInterface, requests []reconcile.Request) {
	for _, req := range requests {
		q.Add(req)
	}
}

// Mapper maps an object to a collection of keys to be enqueued
type Mapper interface {
	// Map maps an object for a create event.
	MapCreate(MapCreateObject) []reconcile.Request
	// Map maps an object for a delete event.
	MapDelete(MapDeleteObject) []reconcile.Request
	// Map maps an object for a generic event.
	MapGeneric(MapGenericObject) []reconcile.Request
	// Map maps an object for an update event.
	MapUpdate(MapUpdateObject) []reconcile.Request
}

// MapGenericObject contains information from a generic event to be transformed into a Request.
type MapGenericObject struct {
	// Meta is the meta data for an object from an event.
	Meta metav1.Object

	// Object is the object from an event.
	Object runtime.Object
}

// MapCreateObject contains information from a create event to be transformed into a Request.
type MapCreateObject struct {
	// Meta is the meta data for an object from an event.
	Meta metav1.Object

	// Object is the object from an event.
	Object runtime.Object
}

// MapDeleteObject contains information from a delete event to be transformed into a Request.
type MapDeleteObject struct {
	// Meta is the meta data for an object from an event.
	Meta metav1.Object

	// Object is the object from an event.
	Object runtime.Object
}

// MapUpdateObject contains information from an update event to be transformed into a Request.
type MapUpdateObject struct {
	// MetaOld is the old meta data for an object from an update event.
	MetaOld metav1.Object
	// ObjectOld is the old object from an update event.
	ObjectOld runtime.Object

	// MetaNew is the new meta data for an object from an update event.
	MetaNew metav1.Object
	// ObjectNew is the new object from an update event.
	ObjectNew runtime.Object
}

// UpdateBehavior determines how an update should be handled.
type UpdateBehavior uint8

const (
	// UpdateWithOldAndNew considers both, the old as well as the new object, in case of an update.
	UpdateWithOldAndNew UpdateBehavior = iota
	// UpdateWithOld considers only the old object in case of an update.
	UpdateWithOld
	// UpdateWithNew considers only the new object in case of an update.
	UpdateWithNew
)

type handlerMapper struct {
	updateBehavior UpdateBehavior
	mapper         handler.Mapper
}

// InjectFunc implements Injector.
func (h *handlerMapper) InjectFunc(f inject.Func) error {
	return f(h.mapper)
}

func (h *handlerMapper) MapCreate(c MapCreateObject) []reconcile.Request {
	return h.mapper.Map(handler.MapObject{Meta: c.Meta, Object: c.Object})
}

func (h *handlerMapper) MapDelete(c MapDeleteObject) []reconcile.Request {
	return h.mapper.Map(handler.MapObject{Meta: c.Meta, Object: c.Object})
}

func (h *handlerMapper) MapGeneric(c MapGenericObject) []reconcile.Request {
	return h.mapper.Map(handler.MapObject{Meta: c.Meta, Object: c.Object})
}

func (h *handlerMapper) MapUpdate(c MapUpdateObject) []reconcile.Request {
	switch h.updateBehavior {
	case UpdateWithOldAndNew:
		var requests []reconcile.Request
		requests = append(requests, h.mapper.Map(handler.MapObject{Meta: c.MetaOld, Object: c.ObjectOld})...)
		requests = append(requests, h.mapper.Map(handler.MapObject{Meta: c.MetaNew, Object: c.ObjectNew})...)
		return requests
	case UpdateWithOld:
		return h.mapper.Map(handler.MapObject{Meta: c.MetaOld, Object: c.ObjectOld})
	case UpdateWithNew:
		return h.mapper.Map(handler.MapObject{Meta: c.MetaNew, Object: c.ObjectNew})
	default:
		return nil
	}
}

// SimpleMapper wraps a mapper and calls its update function according to the given `updateBehavior`.
func SimpleMapper(mapper handler.Mapper, updateBehavior UpdateBehavior) Mapper {
	return &handlerMapper{updateBehavior, mapper}
}
