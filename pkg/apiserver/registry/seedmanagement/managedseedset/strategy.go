// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseedset

import (
	"context"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/validation"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Strategy defines the strategy for storing managedseedsets.
type Strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// NewStrategy defines the storage strategy for ManagedSeedSets.
func NewStrategy() Strategy {
	return Strategy{api.Scheme, names.SimpleNameGenerator}
}

// NamespaceScoped returns true if the object must be within a namespace.
func (Strategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate mutates some fields in the object before it's created.
func (s Strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	managedSeedSet := obj.(*seedmanagement.ManagedSeedSet)

	managedSeedSet.Generation = 1
	managedSeedSet.Status = seedmanagement.ManagedSeedSetStatus{}
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s Strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newManagedSeedSet := obj.(*seedmanagement.ManagedSeedSet)
	oldManagedSeedSet := old.(*seedmanagement.ManagedSeedSet)
	newManagedSeedSet.Status = oldManagedSeedSet.Status

	if mustIncreaseGeneration(oldManagedSeedSet, newManagedSeedSet) {
		newManagedSeedSet.Generation = oldManagedSeedSet.Generation + 1
	}
}

func mustIncreaseGeneration(oldManagedSeedSet, newManagedSeedSet *seedmanagement.ManagedSeedSet) bool {
	// The spec changed
	if !apiequality.Semantic.DeepEqual(oldManagedSeedSet.Spec, newManagedSeedSet.Spec) {
		return true
	}

	// The deletion timestamp was set
	if oldManagedSeedSet.DeletionTimestamp == nil && newManagedSeedSet.DeletionTimestamp != nil {
		return true
	}

	// The operation annotation was added with value "reconcile"
	if kubernetesutils.HasMetaDataAnnotation(&newManagedSeedSet.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile) {
		delete(newManagedSeedSet.Annotations, v1beta1constants.GardenerOperation)
		return true
	}

	return false
}

// Validate validates the given object.
func (Strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	managedSeedSet := obj.(*seedmanagement.ManagedSeedSet)
	return validation.ValidateManagedSeedSet(managedSeedSet)
}

// Canonicalize allows an object to be mutated into a canonical form. This
// ensures that code that operates on these objects can rely on the common
// form for things like comparison.  Canonicalize is invoked after
// validation has succeeded but before the object has been persisted.
// This method may mutate the object.
func (Strategy) Canonicalize(_ runtime.Object) {
}

// AllowCreateOnUpdate returns true if the object can be created by a PUT.
func (Strategy) AllowCreateOnUpdate() bool {
	return false
}

// AllowUnconditionalUpdate returns true if the object can be updated
// unconditionally (irrespective of the latest resource version), when
// there is no resource version specified in the object.
func (Strategy) AllowUnconditionalUpdate() bool {
	return false
}

// ValidateUpdate validates the update on the given old and new object.
func (Strategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldManagedSeedSet, newManagedSeedSet := oldObj.(*seedmanagement.ManagedSeedSet), newObj.(*seedmanagement.ManagedSeedSet)
	return validation.ValidateManagedSeedSetUpdate(newManagedSeedSet, oldManagedSeedSet)
}

// WarningsOnCreate returns warnings to the client performing a create.
func (Strategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (Strategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

// StatusStrategy defines the strategy for storing seeds statuses.
type StatusStrategy struct {
	Strategy
}

// NewStatusStrategy defines the storage strategy for the status subresource of ManagedSeedSets.
func NewStatusStrategy() StatusStrategy {
	return StatusStrategy{NewStrategy()}
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s StatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newManagedSeedSet := obj.(*seedmanagement.ManagedSeedSet)
	oldManagedSeedSet := old.(*seedmanagement.ManagedSeedSet)
	newManagedSeedSet.Spec = oldManagedSeedSet.Spec
}

// ValidateUpdate validates the update on the given old and new object.
func (StatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateManagedSeedSetStatusUpdate(obj.(*seedmanagement.ManagedSeedSet), old.(*seedmanagement.ManagedSeedSet))
}

// MatchManagedSeedSet returns a generic matcher for a given label and field selector.
func MatchManagedSeedSet(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	managedSeedSet, ok := obj.(*seedmanagement.ManagedSeedSet)
	if !ok {
		return nil, nil, fmt.Errorf("not a ManagedSeedSet")
	}
	return labels.Set(managedSeedSet.Labels), ToSelectableFields(managedSeedSet), nil
}

// ToSelectableFields returns a field set that represents the object.
func ToSelectableFields(managedSeedSet *seedmanagement.ManagedSeedSet) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	fieldsSet := make(fields.Set, 2)
	return generic.AddObjectMetaFieldsSet(fieldsSet, &managedSeedSet.ObjectMeta, true)
}
