// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootstate

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"
)

type shootStateStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for ShootState.
var Strategy = shootStateStrategy{api.Scheme, names.SimpleNameGenerator}

func (shootStateStrategy) NamespaceScoped() bool {
	return true
}

func (shootStateStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	shootState := obj.(*core.ShootState)

	shootState.Generation = 1
}

func (shootStateStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newShootState := obj.(*core.ShootState)
	oldShootState := old.(*core.ShootState)

	if mustIncreaseGeneration(oldShootState, newShootState) {
		newShootState.Generation = oldShootState.Generation + 1
	}
}

func mustIncreaseGeneration(oldShootState, newShootState *core.ShootState) bool {
	// The ShootState specification changes.
	if !apiequality.Semantic.DeepEqual(oldShootState.Spec, newShootState.Spec) {
		return true
	}

	// The deletion timestamp was set.
	if oldShootState.DeletionTimestamp == nil && newShootState.DeletionTimestamp != nil {
		return true
	}

	return false
}

func (shootStateStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	shootState := obj.(*core.ShootState)
	return validation.ValidateShootState(shootState)
}

func (shootStateStrategy) Canonicalize(_ runtime.Object) {
}

func (shootStateStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (shootStateStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	newShootState := newObj.(*core.ShootState)
	oldShootState := oldObj.(*core.ShootState)
	return validation.ValidateShootStateUpdate(newShootState, oldShootState)
}

func (shootStateStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings to the client performing a create.
func (shootStateStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (shootStateStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}
