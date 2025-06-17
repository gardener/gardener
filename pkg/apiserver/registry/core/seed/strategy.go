// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/validation"
)

// Strategy defines the strategy for storing seeds.
type Strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// NewStrategy defines the storage strategy for Seeds.
func NewStrategy() Strategy {
	return Strategy{api.Scheme, names.SimpleNameGenerator}
}

// NamespaceScoped returns true if the object must be within a namespace.
func (Strategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate mutates some fields in the object before it's created.
func (s Strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	seed := obj.(*core.Seed)

	seed.Generation = 1
	seed.Status = core.SeedStatus{}
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s Strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newSeed := obj.(*core.Seed)
	oldSeed := old.(*core.Seed)
	newSeed.Status = oldSeed.Status

	if mustIncreaseGeneration(oldSeed, newSeed) {
		newSeed.Generation = oldSeed.Generation + 1
	}
}

// Canonicalize can be used to transform the object into its canonical format.
func (Strategy) Canonicalize(_ runtime.Object) {
}

func mustIncreaseGeneration(oldSeed, newSeed *core.Seed) bool {
	// The spec changed
	if !apiequality.Semantic.DeepEqual(oldSeed.Spec, newSeed.Spec) {
		return true
	}

	// The deletion timestamp was set
	if oldSeed.DeletionTimestamp == nil && newSeed.DeletionTimestamp != nil {
		return true
	}

	// bump the generation in case certain operations were triggered
	if oldSeed.Annotations[v1beta1constants.GardenerOperation] != newSeed.Annotations[v1beta1constants.GardenerOperation] {
		switch newSeed.Annotations[v1beta1constants.GardenerOperation] {
		case v1beta1constants.SeedOperationRenewGardenAccessSecrets:
			return true
		case v1beta1constants.SeedOperationRenewWorkloadIdentityTokens:
			return true
		case v1beta1constants.GardenerOperationReconcile:
			delete(newSeed.Annotations, v1beta1constants.GardenerOperation)
			return true
		case v1beta1constants.GardenerOperationRenewKubeconfig:
			return true
		}
	}

	return false
}

// Validate validates the given object.
func (Strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	seed := obj.(*core.Seed)
	return validation.ValidateSeed(seed)
}

// AllowCreateOnUpdate returns true if the object can be created by a PUT.
func (Strategy) AllowCreateOnUpdate() bool {
	return false
}

// AllowUnconditionalUpdate returns true if the object can be updated
// unconditionally (irrespective of the latest resource version), when
// there is no resource version specified in the object.
func (Strategy) AllowUnconditionalUpdate() bool {
	return true
}

// ValidateUpdate validates the update on the given old and new object.
func (Strategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldSeed, newSeed := oldObj.(*core.Seed), newObj.(*core.Seed)
	return validation.ValidateSeedUpdate(newSeed, oldSeed)
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

// NewStatusStrategy defines the storage strategy for the status subresource of Seeds.
func NewStatusStrategy() StatusStrategy {
	return StatusStrategy{NewStrategy()}
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s StatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newSeed := obj.(*core.Seed)
	oldSeed := old.(*core.Seed)
	newSeed.Spec = oldSeed.Spec
}

// ValidateUpdate validates the update on the given old and new object.
func (StatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateSeedStatusUpdate(obj.(*core.Seed), old.(*core.Seed))
}
