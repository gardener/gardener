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

package shootleftover

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
)

// Strategy defines the strategy for storing shootleftovers.
type Strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// NewStrategy defines the storage strategy for ShootLeftovers.
func NewStrategy() Strategy {
	return Strategy{api.Scheme, names.SimpleNameGenerator}
}

// NamespaceScoped returns true if the object must be within a namespace.
func (Strategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate mutates some fields in the object before it's created.
func (s Strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	shootLeftover := obj.(*core.ShootLeftover)

	shootLeftover.Generation = 1
	shootLeftover.Status = core.ShootLeftoverStatus{}
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s Strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newShootLeftover := obj.(*core.ShootLeftover)
	oldShootLeftover := old.(*core.ShootLeftover)
	newShootLeftover.Status = oldShootLeftover.Status

	if mustIncreaseGeneration(oldShootLeftover, newShootLeftover) {
		newShootLeftover.Generation = oldShootLeftover.Generation + 1
	}
}

func mustIncreaseGeneration(oldShootLeftover, newShootLeftover *core.ShootLeftover) bool {
	// The spec changed
	if !apiequality.Semantic.DeepEqual(oldShootLeftover.Spec, newShootLeftover.Spec) {
		return true
	}

	// The deletion timestamp was set
	if oldShootLeftover.DeletionTimestamp == nil && newShootLeftover.DeletionTimestamp != nil {
		return true
	}

	return false
}

// Validate validates the given object.
func (Strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	shootLeftover := obj.(*core.ShootLeftover)
	return validation.ValidateShootLeftover(shootLeftover)
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
	oldShootLeftover, newShootLeftover := oldObj.(*core.ShootLeftover), newObj.(*core.ShootLeftover)
	return validation.ValidateShootLeftoverUpdate(newShootLeftover, oldShootLeftover)
}

// WarningsOnCreate returns warnings to the client performing a create.
func (Strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (Strategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

// StatusStrategy defines the strategy for storing seeds statuses.
type StatusStrategy struct {
	Strategy
}

// NewStatusStrategy defines the storage strategy for the status subresource of ShootLeftovers.
func NewStatusStrategy() StatusStrategy {
	return StatusStrategy{NewStrategy()}
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s StatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newShootLeftover := obj.(*core.ShootLeftover)
	oldShootLeftover := old.(*core.ShootLeftover)
	newShootLeftover.Spec = oldShootLeftover.Spec
}

// ValidateUpdate validates the update on the given old and new object.
func (StatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateShootLeftoverStatusUpdate(obj.(*core.ShootLeftover), old.(*core.ShootLeftover))
}

// MatchShootLeftover returns a generic matcher for a given label and field selector.
func MatchShootLeftover(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:       label,
		Field:       field,
		GetAttrs:    GetAttrs,
		IndexFields: []string{core.ShootLeftoverSeedName},
	}
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	shootLeftover, ok := obj.(*core.ShootLeftover)
	if !ok {
		return nil, nil, fmt.Errorf("not a ShootLeftover")
	}
	return labels.Set(shootLeftover.ObjectMeta.Labels), ToSelectableFields(shootLeftover), nil
}

// ToSelectableFields returns a field set that represents the object.
func ToSelectableFields(shootLeftover *core.ShootLeftover) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	fieldsSet := make(fields.Set, 3)
	fieldsSet[core.ShootLeftoverSeedName] = GetSeedName(shootLeftover)
	return generic.AddObjectMetaFieldsSet(fieldsSet, &shootLeftover.ObjectMeta, true)
}

// SeedNameTriggerFunc returns spec.seedName of the given ShootLeftover.
func SeedNameTriggerFunc(obj runtime.Object) string {
	shootLeftover, ok := obj.(*core.ShootLeftover)
	if !ok {
		return ""
	}

	return GetSeedName(shootLeftover)
}

// GetSeedName returns spec.seedName of the given ShootLeftover.
func GetSeedName(shootLeftover *core.ShootLeftover) string {
	return shootLeftover.Spec.SeedName
}
