/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package predicate

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// Predicate filters events before enqueuing the keys.
type Predicate = TypedPredicate[client.Object]

// TypedPredicate filters events before enqueuing the keys.
type TypedPredicate[object any] interface {
	// Create returns true if the Create event should be processed
	Create(event.TypedCreateEvent[object]) bool

	// Delete returns true if the Delete event should be processed
	Delete(event.TypedDeleteEvent[object]) bool

	// Update returns true if the Update event should be processed
	Update(event.TypedUpdateEvent[object]) bool

	// Generic returns true if the Generic event should be processed
	Generic(event.TypedGenericEvent[object]) bool
}

var _ Predicate = Funcs{}

// Funcs is a function that implements Predicate.
type Funcs = TypedFuncs[client.Object]

// TypedFuncs is a function that implements TypedPredicate.
type TypedFuncs[object any] struct {
	// Create returns true if the Create event should be processed
	CreateFunc func(event.TypedCreateEvent[object]) bool

	// Delete returns true if the Delete event should be processed
	DeleteFunc func(event.TypedDeleteEvent[object]) bool

	// Update returns true if the Update event should be processed
	UpdateFunc func(event.TypedUpdateEvent[object]) bool

	// Generic returns true if the Generic event should be processed
	GenericFunc func(event.TypedGenericEvent[object]) bool
}

// Create implements Predicate.
func (p TypedFuncs[object]) Create(e event.TypedCreateEvent[object]) bool {
	if p.CreateFunc != nil {
		return p.CreateFunc(e)
	}
	return true
}

// Delete implements Predicate.
func (p TypedFuncs[object]) Delete(e event.TypedDeleteEvent[object]) bool {
	if p.DeleteFunc != nil {
		return p.DeleteFunc(e)
	}
	return true
}

// Update implements Predicate.
func (p TypedFuncs[object]) Update(e event.TypedUpdateEvent[object]) bool {
	if p.UpdateFunc != nil {
		return p.UpdateFunc(e)
	}
	return true
}

// Generic implements Predicate.
func (p TypedFuncs[object]) Generic(e event.TypedGenericEvent[object]) bool {
	if p.GenericFunc != nil {
		return p.GenericFunc(e)
	}
	return true
}

// NewPredicateFuncs returns a predicate funcs that applies the given filter function
// on CREATE, UPDATE, DELETE and GENERIC events. For UPDATE events, the filter is applied
// to the new object.
func NewPredicateFuncs(filter func(object client.Object) bool) Funcs {
	return Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return filter(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return filter(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return filter(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return filter(e.Object)
		},
	}
}

// NewTypedPredicateFuncs returns a predicate funcs that applies the given filter function
// on CREATE, UPDATE, DELETE and GENERIC events. For UPDATE events, the filter is applied
// to the new object.
func NewTypedPredicateFuncs[object any](filter func(object object) bool) TypedFuncs[object] {
	return TypedFuncs[object]{
		CreateFunc: func(e event.TypedCreateEvent[object]) bool {
			return filter(e.Object)
		},
		UpdateFunc: func(e event.TypedUpdateEvent[object]) bool {
			return filter(e.ObjectNew)
		},
		DeleteFunc: func(e event.TypedDeleteEvent[object]) bool {
			return filter(e.Object)
		},
		GenericFunc: func(e event.TypedGenericEvent[object]) bool {
			return filter(e.Object)
		},
	}
}
