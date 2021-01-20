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

package managedseed

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/validation"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
)

// Strategy defines the strategy for storing managedseeds.
type Strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// NewStrategy defines the storage strategy for ManagedSeeds.
func NewStrategy() Strategy {
	return Strategy{api.Scheme, names.SimpleNameGenerator}
}

// NamespaceScoped returns true if the object must be within a namespace.
func (Strategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate mutates some fields in the object before it's created.
func (s Strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	managedSeed := obj.(*seedmanagement.ManagedSeed)

	managedSeed.Generation = 1
	managedSeed.Status = seedmanagement.ManagedSeedStatus{}
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s Strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newManagedSeed := obj.(*seedmanagement.ManagedSeed)
	oldManagedSeed := old.(*seedmanagement.ManagedSeed)
	newManagedSeed.Status = oldManagedSeed.Status

	if !apiequality.Semantic.DeepEqual(oldManagedSeed.Spec, newManagedSeed.Spec) ||
		oldManagedSeed.DeletionTimestamp == nil && newManagedSeed.DeletionTimestamp != nil {
		newManagedSeed.Generation = oldManagedSeed.Generation + 1
	}
}

// Validate validates the given object.
func (Strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	managedSeed := obj.(*seedmanagement.ManagedSeed)
	return validation.ValidateManagedSeed(managedSeed)
}

// Canonicalize allows an object to be mutated into a canonical form. This
// ensures that code that operates on these objects can rely on the common
// form for things like comparison.  Canonicalize is invoked after
// validation has succeeded but before the object has been persisted.
// This method may mutate the object.
func (Strategy) Canonicalize(obj runtime.Object) {
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
func (Strategy) ValidateUpdate(ctx context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldManagedSeed, newManagedSeed := oldObj.(*seedmanagement.ManagedSeed), newObj.(*seedmanagement.ManagedSeed)
	return validation.ValidateManagedSeedUpdate(newManagedSeed, oldManagedSeed)
}

// StatusStrategy defines the strategy for storing seeds statuses.
type StatusStrategy struct {
	Strategy
}

// NewStatusStrategy defines the storage strategy for the status subresource of ManagedSeeds.
func NewStatusStrategy() StatusStrategy {
	return StatusStrategy{NewStrategy()}
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s StatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newManagedSeed := obj.(*seedmanagement.ManagedSeed)
	oldManagedSeed := old.(*seedmanagement.ManagedSeed)
	newManagedSeed.Spec = oldManagedSeed.Spec
}

// ValidateUpdate validates the update on the given old and new object.
func (StatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateManagedSeedStatusUpdate(obj.(*seedmanagement.ManagedSeed), old.(*seedmanagement.ManagedSeed))
}

// MatchManagedSeed returns a generic matcher for a given label and field selector.
func MatchManagedSeed(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:       label,
		Field:       field,
		GetAttrs:    GetAttrs,
		IndexFields: []string{seedmanagement.ManagedSeedShootName},
	}
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	managedSeed, ok := obj.(*seedmanagement.ManagedSeed)
	if !ok {
		return nil, nil, fmt.Errorf("not a ManagedSeed")
	}
	return labels.Set(managedSeed.ObjectMeta.Labels), ToSelectableFields(managedSeed), nil
}

// ToSelectableFields returns a field set that represents the object.
func ToSelectableFields(managedSeed *seedmanagement.ManagedSeed) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	shootSpecificFieldsSet := make(fields.Set, 3)
	shootSpecificFieldsSet[seedmanagement.ManagedSeedShootName] = managedSeed.Spec.Shoot.Name
	return generic.AddObjectMetaFieldsSet(shootSpecificFieldsSet, &managedSeed.ObjectMeta, true)
}

// ShootNameTriggerFunc returns spec.shoot.name of the given ManagedSeed.
func ShootNameTriggerFunc(obj runtime.Object) string {
	managedSeed, ok := obj.(*seedmanagement.ManagedSeed)
	if !ok {
		return ""
	}

	return managedSeed.Spec.Shoot.Name
}
