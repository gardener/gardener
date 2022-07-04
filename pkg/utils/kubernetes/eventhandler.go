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

package kubernetes

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ControllerPredicateFactory provides a method for creating new Predicates for a controller.
type ControllerPredicateFactory interface {
	// NewControllerPredicate creates and returns a new Predicate with the given controller.
	NewControllerPredicate(client.Object) predicate.Predicate
}

// ControllerPredicateFactoryFunc is a function that implements ControllerPredicateFactory.
type ControllerPredicateFactoryFunc func(client.Object, client.Object, client.Object, bool) bool

// NewControllerPredicate creates and returns a new Predicate with the given controller.
func (f ControllerPredicateFactoryFunc) NewControllerPredicate(controller client.Object) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return f(e.Object, nil, controller, false)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return f(e.ObjectNew, e.ObjectOld, controller, false)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return f(e.Object, nil, controller, true)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

// Enqueuer provides a method for enqueuing an object for processing.
type Enqueuer interface {
	// Enqueue enqueues the object for processing.
	Enqueue(client.Object)
}

// EnqueuerFunc is a function that implements Enqueuer.
type EnqueuerFunc func(client.Object)

// Enqueue enqueues the object for processing.
func (f EnqueuerFunc) Enqueue(obj client.Object) {
	f(obj)
}

// ControlledResourceEventHandler is an implementation of cache.ResourceEventHandler that enqueues controllers upon
// controlled resource events, if a predicate check is successful.
type ControlledResourceEventHandler struct {
	// ControllerTypes is a list of controller types. If multiple controller types are specified,
	// there is a chain of controllers. Only the top-most controller (with the last listed type) is eventually enqueued.
	ControllerTypes []ControllerType
	// Ctx is the context used to get controller objects.
	Ctx context.Context
	// Reader is the reader used to get controller objects.
	Reader client.Reader
	// ControllerPredicateFactory is used to create a predicate to check if an object event is of interest to a controller,
	// before enqueueing it. If nil, the controller is always enqueued.
	ControllerPredicateFactory ControllerPredicateFactory
	// Enqueuer is used to enqueue the controller.
	Enqueuer Enqueuer
	// Scheme is used to resolve types to their GroupKinds.
	Scheme *runtime.Scheme
	// Logger is used to log messages.
	Logger logr.Logger
}

// ControllerType contains information about a controller type.
type ControllerType struct {
	// Type is the controller type. It is used to check the controller group and kind and get objects of the specified type.
	Type client.Object
	// Namespace is an optional namespace to look for controllers. If nil, the namespace of the controlled object is used.
	Namespace *string
	// NameFunc is an optional function that returns the controller name from the given object.
	// It is only used if the object doesn't have a controller ref.
	NameFunc func(obj client.Object) string

	// groupKind is the cached GroupKind as determined from Type.
	groupKind *schema.GroupKind
}

// OnAdd is called when an object is added.
func (h *ControlledResourceEventHandler) OnAdd(o interface{}) {
	obj, ok := o.(client.Object)
	if !ok {
		return
	}

	// Get the controller, if any
	controller := h.getControllerOf(obj, 0)
	if controller == nil {
		return
	}

	// Create and check predicate to determine if the event is of interest to the controller
	if h.ControllerPredicateFactory != nil {
		e := event.CreateEvent{Object: obj}
		if p := h.ControllerPredicateFactory.NewControllerPredicate(controller); !p.Create(e) {
			return
		}
	}

	// Enqueue the controller
	h.logEnqueue(controller, obj, "Add")
	h.Enqueuer.Enqueue(controller)
}

// OnUpdate is called when an object is modified.
func (h *ControlledResourceEventHandler) OnUpdate(old, new interface{}) {
	oldObj, ok := old.(client.Object)
	if !ok {
		return
	}
	newObj, ok := new.(client.Object)
	if !ok {
		return
	}

	// Get the old and the new controllers, if any
	oldController := h.getControllerOf(oldObj, 0)
	newController := h.getControllerOf(newObj, 0)

	// If the controller has changed, enqueue the old controller, if any
	if oldController != nil && (newController == nil || oldController.GetUID() != newController.GetUID()) {
		// Create and check predicate to determine if the event is of interest to the old controller
		if h.ControllerPredicateFactory != nil {
			e := event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}
			if p := h.ControllerPredicateFactory.NewControllerPredicate(oldController); !p.Update(e) {
				return
			}
		}

		// Enqueue the old controller
		h.logEnqueue(oldController, oldObj, "Update")
		h.Enqueuer.Enqueue(oldController)
	}

	if newController == nil {
		return
	}

	// Create and check predicate to determine if the event is of interest to the new controller
	if h.ControllerPredicateFactory != nil {
		e := event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}
		if p := h.ControllerPredicateFactory.NewControllerPredicate(newController); !p.Update(e) {
			return
		}
	}

	// Enqueue the new controller
	h.logEnqueue(newController, newObj, "Update")
	h.Enqueuer.Enqueue(newController)
}

// OnDelete is called when an object is deleted. It will get the final state of the item if it is known,
// otherwise it will get an object of type DeletedFinalStateUnknown.
func (h *ControlledResourceEventHandler) OnDelete(o interface{}) {
	obj, ok := o.(client.Object)
	if !ok {
		tombstone, ok := o.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		obj, ok = tombstone.Obj.(client.Object)
		if !ok {
			return
		}
	}

	// Get the controller, if any
	controller := h.getControllerOf(obj, 0)
	if controller == nil {
		return
	}

	// Create and check predicate to determine if the event is of interest to the controller
	if h.ControllerPredicateFactory != nil {
		e := event.DeleteEvent{Object: obj}
		if p := h.ControllerPredicateFactory.NewControllerPredicate(controller); !p.Delete(e) {
			return
		}
	}

	// Enqueue the controller
	h.logEnqueue(controller, obj, "Delete")
	h.Enqueuer.Enqueue(controller)
}

func (h *ControlledResourceEventHandler) getControllerOf(obj client.Object, index int) client.Object {
	ct := h.ControllerTypes[index]

	// Get controller by ref or by name
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef != nil {
		return h.getControllerByRef(obj.GetNamespace(), controllerRef, index)
	} else if ct.NameFunc != nil {
		if name := ct.NameFunc(obj); name != "" {
			return h.getControllerByName(obj.GetNamespace(), name, index)
		}
	}
	return nil
}

func (h *ControlledResourceEventHandler) getControllerByName(namespace, name string, index int) client.Object {
	ct := h.ControllerTypes[index]

	// Get controller by name
	controller := ct.Type.DeepCopyObject().(client.Object)
	if ct.Namespace != nil {
		namespace = *ct.Namespace
	}
	if err := h.Reader.Get(h.Ctx, Key(namespace, name), controller); err != nil {
		return nil
	}

	// If this is the final controller in the chain, return it, otherwise move up the chain
	if index == len(h.ControllerTypes)-1 {
		return controller
	}
	return h.getControllerOf(controller, index+1)
}

func (h *ControlledResourceEventHandler) getControllerByRef(namespace string, controllerRef *metav1.OwnerReference, index int) client.Object {
	ct := h.ControllerTypes[index]

	// Check controller ref group and kind
	crgv, err := schema.ParseGroupVersion(controllerRef.APIVersion)
	if err != nil {
		return nil
	}
	gk, err := h.getGroupKind(&ct)
	if err != nil {
		return nil
	}
	if crgv.Group != gk.Group || controllerRef.Kind != gk.Kind {
		return nil
	}

	// Get controller by name
	controller := ct.Type.DeepCopyObject().(client.Object)
	if ct.Namespace != nil {
		namespace = *ct.Namespace
	}
	if err := h.Reader.Get(h.Ctx, Key(namespace, controllerRef.Name), controller); err != nil {
		return nil
	}

	// Check UID
	if controller.GetUID() != controllerRef.UID {
		return nil
	}

	// If this is the final controller in the chain, return it, otherwise move up the chain
	if index == len(h.ControllerTypes)-1 {
		return controller
	}
	return h.getControllerOf(controller, index+1)
}

func (h *ControlledResourceEventHandler) logEnqueue(controller, obj client.Object, eventType string) {
	h.Logger.V(1).Info("Enqueuing controlling object due to change to controlled object",
		"controllingKind", h.getLastControllerKind(), "controllingObject", client.ObjectKeyFromObject(controller),
		"controlledKind", h.getObjectKind(obj), "controlledObject", client.ObjectKeyFromObject(obj),
		"eventType", eventType,
	)
}

func (h *ControlledResourceEventHandler) getGroupKind(ct *ControllerType) (*schema.GroupKind, error) {
	if ct.groupKind == nil {
		var err error
		if ct.groupKind, err = getObjectGroupKind(h.Scheme, ct.Type); err != nil {
			return nil, err
		}
	}
	return ct.groupKind, nil
}

func (h *ControlledResourceEventHandler) getLastControllerKind() string {
	if gk, err := h.getGroupKind(&h.ControllerTypes[len(h.ControllerTypes)-1]); err == nil {
		return gk.Kind
	}
	return ""
}

func (h *ControlledResourceEventHandler) getObjectKind(obj client.Object) string {
	if gk, err := getObjectGroupKind(h.Scheme, obj); err == nil {
		return gk.Kind
	}
	return ""
}

// getObjectGroupKind parses the given object into a schema.GroupKind.
func getObjectGroupKind(scheme *runtime.Scheme, obj client.Object) (*schema.GroupKind, error) {
	// Get the kinds of the type
	kinds, _, err := scheme.ObjectKinds(obj)
	if err != nil {
		return nil, err
	}
	if len(kinds) != 1 {
		return nil, fmt.Errorf("expected exactly 1 kind for type %T, but found %d kinds", obj, len(kinds))
	}
	return &schema.GroupKind{Group: kinds[0].Group, Kind: kinds[0].Kind}, nil
}
