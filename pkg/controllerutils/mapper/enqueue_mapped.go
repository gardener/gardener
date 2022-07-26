// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package mapper

import (
	"context"
	"fmt"

	ctxutils "github.com/gardener/gardener/pkg/utils/context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

// Mapper maps an object to a collection of keys to be enqueued
type Mapper interface {
	// Map maps an object
	Map(ctx context.Context, log logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request
}

var _ Mapper = MapFunc(nil)

// MapFunc is the signature required for enqueueing requests from a generic function.
// This type is usually used with EnqueueRequestsFromMapFunc when registering an event mapper.
type MapFunc func(ctx context.Context, log logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request

// Map implements Mapper.
func (f MapFunc) Map(ctx context.Context, log logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
	return f(ctx, log, reader, obj)
}

// EnqueueRequestsFrom is similar to controller-runtime's mapper.EnqueueRequestsFromMapFunc.
// Instead of taking only a MapFunc it also allows passing a Mapper interface. Also, it allows customizing the
// behavior on UpdateEvents.
// For UpdateEvents, the given UpdateBehavior decides if only the old, only the new or both objects should be mapped
// and enqueued.
func EnqueueRequestsFrom(m Mapper, updateBehavior UpdateBehavior, log logr.Logger) handler.EventHandler {
	return &enqueueRequestsFromMapFunc{
		mapper:         m,
		updateBehavior: updateBehavior,
		log:            log,
	}
}

type enqueueRequestsFromMapFunc struct {
	// mapper transforms the argument into a slice of keys to be reconciled
	mapper Mapper
	// updateBehavior decides which object(s) to map and enqueue on updates
	updateBehavior UpdateBehavior

	ctx    context.Context
	log    logr.Logger
	reader client.Reader
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

func (e *enqueueRequestsFromMapFunc) InjectCache(c cache.Cache) error {
	e.reader = c
	return nil
}

func (e *enqueueRequestsFromMapFunc) InjectStopChannel(stopCh <-chan struct{}) error {
	e.ctx = ctxutils.FromStopChannel(stopCh)
	return nil
}

func (e *enqueueRequestsFromMapFunc) InjectFunc(f inject.Func) error {
	if f == nil {
		return nil
	}
	return f(e.mapper)
}

func (e *enqueueRequestsFromMapFunc) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.mapAndEnqueue(q, evt.Object)
}

func (e *enqueueRequestsFromMapFunc) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	switch e.updateBehavior {
	case UpdateWithOldAndNew:
		e.mapAndEnqueue(q, evt.ObjectOld)
		e.mapAndEnqueue(q, evt.ObjectNew)
	case UpdateWithOld:
		e.mapAndEnqueue(q, evt.ObjectOld)
	case UpdateWithNew:
		e.mapAndEnqueue(q, evt.ObjectNew)
	}
}

func (e *enqueueRequestsFromMapFunc) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.mapAndEnqueue(q, evt.Object)
}

func (e *enqueueRequestsFromMapFunc) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.mapAndEnqueue(q, evt.Object)
}

func (e *enqueueRequestsFromMapFunc) mapAndEnqueue(q workqueue.RateLimitingInterface, object client.Object) {
	for _, req := range e.mapper.Map(e.ctx, e.log, e.reader, object) {
		q.Add(req)
	}
}

// ObjectListToRequests adds a reconcile.Request for each object in the provided list.
func ObjectListToRequests(list client.ObjectList, predicates ...func(client.Object) bool) []reconcile.Request {
	var requests []reconcile.Request

	utilruntime.HandleError(meta.EachListItem(list, func(object runtime.Object) error {
		obj, ok := object.(client.Object)
		if !ok {
			return fmt.Errorf("cannot convert object of type %T to client.Object", object)
		}

		for _, predicate := range predicates {
			if !predicate(obj) {
				return nil
			}
		}

		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		}})

		return nil
	}))

	return requests
}
