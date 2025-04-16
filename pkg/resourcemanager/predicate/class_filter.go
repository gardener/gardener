// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package predicate

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
)

const (
	// FinalizerName is the finalizer base name that is injected into ManagedResources.
	// The concrete finalizer is finally containing this base name and the resource class.
	FinalizerName = "resources.gardener.cloud/gardener-resource-manager"
)

// ClassFilter keeps the resource class for the actual controller instance
// and is used as Filter predicate for events finally passed to the controller.
// Only objects that have the same class as the controller
// or their resources deletion is handled by the controller are filtered.
type ClassFilter struct {
	resourceClass string

	objectFinalizer string
}

var _ predicate.Predicate = &ClassFilter{}

// NewClassFilter returns a new `ClassFilter` instance.
func NewClassFilter(class string) *ClassFilter {
	if class == "" {
		class = resourcemanagerconfigv1alpha1.DefaultResourceClass
	}

	finalizer := FinalizerName + "-" + class
	if class == resourcemanagerconfigv1alpha1.DefaultResourceClass || class == resourcemanagerconfigv1alpha1.AllResourceClass {
		finalizer = FinalizerName
	}

	return &ClassFilter{
		resourceClass:   class,
		objectFinalizer: finalizer,
	}
}

// ResourceClass returns the actually configured resource class
func (f *ClassFilter) ResourceClass() string {
	return f.resourceClass
}

// FinalizerName determines the finalizer name to be used for the actual resource class
func (f *ClassFilter) FinalizerName() string {
	return f.objectFinalizer
}

// Responsible checks whether an object should be managed by the actual controller instance
func (f *ClassFilter) Responsible(o runtime.Object) bool {
	if f.resourceClass == resourcemanagerconfigv1alpha1.AllResourceClass {
		return true
	}

	r := o.(*resourcesv1alpha1.ManagedResource)
	c := ptr.Deref(r.Spec.Class, "")
	return c == f.resourceClass || (c == "" && f.resourceClass == resourcemanagerconfigv1alpha1.DefaultResourceClass)
}

// IsTransferringResponsibility checks if a Managed Resource has changed its class and should have its resources cleaned by the given controller instance.
func (f *ClassFilter) IsTransferringResponsibility(mr *resourcesv1alpha1.ManagedResource) bool {
	return controllerutil.ContainsFinalizer(mr, f.objectFinalizer) && !f.Responsible(mr)
}

// IsWaitForCleanupRequired checks if a Managed Resource has changed its class and a given controller instance should wait for its resources to be cleaned.
func (f *ClassFilter) IsWaitForCleanupRequired(mr *resourcesv1alpha1.ManagedResource) bool {
	for _, finalizer := range mr.GetFinalizers() {
		if strings.HasPrefix(finalizer, FinalizerName) {
			// mr has a controller responsible for its resources deletion
			return f.Responsible(mr) && finalizer != f.objectFinalizer
		}
	}
	// mr doesn't have a controller responsible for its resources deletion
	return false
}

// Create implements `predicate.Predicate`.
func (f *ClassFilter) Create(e event.CreateEvent) bool {
	return controllerutil.ContainsFinalizer(e.Object, f.objectFinalizer) || f.Responsible(e.Object)
}

// Delete implements `predicate.Predicate`.
func (f *ClassFilter) Delete(e event.DeleteEvent) bool {
	return controllerutil.ContainsFinalizer(e.Object, f.objectFinalizer) || f.Responsible(e.Object)
}

// Update implements `predicate.Predicate`.
func (f *ClassFilter) Update(e event.UpdateEvent) bool {
	return controllerutil.ContainsFinalizer(e.ObjectNew, f.objectFinalizer) || f.Responsible(e.ObjectNew)
}

// Generic implements `predicate.Predicate`.
func (f *ClassFilter) Generic(e event.GenericEvent) bool {
	return controllerutil.ContainsFinalizer(e.Object, f.objectFinalizer) || f.Responsible(e.Object)
}
