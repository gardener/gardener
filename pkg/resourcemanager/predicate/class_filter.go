// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package predicate

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	resourcemanagerv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
)

const (
	// FinalizerName is the finalizer base name that is injected into ManagedResources.
	// The concrete finalizer is finally containing this base name and the resource class.
	FinalizerName = "resources.gardener.cloud/gardener-resource-manager"
)

// ClassFilter keeps the resource class for the actual controller instance
// and is used as Filter predicate for events finally passed to the controller
type ClassFilter struct {
	resourceClass string

	finalizer string
}

var _ predicate.Predicate = &ClassFilter{}

// NewClassFilter returns a new `ClassFilter` instance.
func NewClassFilter(class string) *ClassFilter {
	if class == "" {
		class = resourcemanagerv1alpha1.DefaultResourceClass
	}

	finalizer := FinalizerName + "-" + class
	if class == resourcemanagerv1alpha1.DefaultResourceClass {
		finalizer = FinalizerName
	}

	return &ClassFilter{
		resourceClass: class,
		finalizer:     finalizer,
	}
}

// ResourceClass returns the actually configured resource class
func (f *ClassFilter) ResourceClass() string {
	return f.resourceClass
}

// FinalizerName determines the finalizer name to be used for the actual resource class
func (f *ClassFilter) FinalizerName() string {
	return f.finalizer
}

// Responsible checks whether an object should be managed by the actual controller instance
func (f *ClassFilter) Responsible(o runtime.Object) bool {
	r := o.(*resourcesv1alpha1.ManagedResource)
	c := ""
	if r.Spec.Class != nil && *r.Spec.Class != "" {
		c = *r.Spec.Class
	}
	return c == f.resourceClass || (c == "" && f.resourceClass == resourcemanagerv1alpha1.DefaultResourceClass)
}

// Active checks whether a dedicated object must be handled by the actual controller
// instance. This is split into two conditions. An object must be handled
// if it has already been handled, indicated by the actual finalizer, or
// if the actual controller is responsible for the object.
func (f *ClassFilter) Active(o runtime.Object) (action bool, responsible bool) {
	busy := false
	responsible = f.Responsible(o)
	r := o.(*resourcesv1alpha1.ManagedResource)

	for _, finalizer := range r.GetFinalizers() {
		if strings.HasPrefix(finalizer, FinalizerName) {
			busy = true
			if finalizer == f.finalizer {
				action = true
				return
			}
		}
	}
	action = !busy && responsible
	return
}

// Create implements `predicate.Predicate`.
func (f *ClassFilter) Create(e event.CreateEvent) bool {
	a, r := f.Active(e.Object)
	return a || r
}

// Delete implements `predicate.Predicate`.
func (f *ClassFilter) Delete(e event.DeleteEvent) bool {
	a, r := f.Active(e.Object)
	return a || r
}

// Update implements `predicate.Predicate`.
func (f *ClassFilter) Update(e event.UpdateEvent) bool {
	a, r := f.Active(e.ObjectNew)
	return a || r
}

// Generic implements `predicate.Predicate`.
func (f *ClassFilter) Generic(e event.GenericEvent) bool {
	a, r := f.Active(e.Object)
	return a || r
}
