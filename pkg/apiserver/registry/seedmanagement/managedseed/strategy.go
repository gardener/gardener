// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseed

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
	"github.com/gardener/gardener/pkg/apiserver/registry/seedmanagement/internal/utils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
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
func (s Strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	managedSeed := obj.(*seedmanagement.ManagedSeed)

	managedSeed.Generation = 1
	managedSeed.Status = seedmanagement.ManagedSeedStatus{}

	SyncSeedBackupCredentials(managedSeed)
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s Strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newManagedSeed := obj.(*seedmanagement.ManagedSeed)
	oldManagedSeed := old.(*seedmanagement.ManagedSeed)
	newManagedSeed.Status = oldManagedSeed.Status

	SyncSeedBackupCredentials(newManagedSeed)

	if mustIncreaseGeneration(oldManagedSeed, newManagedSeed) {
		newManagedSeed.Generation = oldManagedSeed.Generation + 1
	}
}

func mustIncreaseGeneration(oldManagedSeed, newManagedSeed *seedmanagement.ManagedSeed) bool {
	// The spec changed
	if !apiequality.Semantic.DeepEqual(oldManagedSeed.Spec, newManagedSeed.Spec) {
		return true
	}

	// The deletion timestamp was set
	if oldManagedSeed.DeletionTimestamp == nil && newManagedSeed.DeletionTimestamp != nil {
		return true
	}

	// The operation annotation was added with value "reconcile"
	if kubernetesutils.HasMetaDataAnnotation(&newManagedSeed.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile) {
		delete(newManagedSeed.Annotations, v1beta1constants.GardenerOperation)
		return true
	}

	// The operation annotation was added with value "renew-kubeconfig"
	if kubernetesutils.HasMetaDataAnnotation(&newManagedSeed.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRenewKubeconfig) {
		return true
	}

	return false
}

// Validate validates the given object.
func (Strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	managedSeed := obj.(*seedmanagement.ManagedSeed)
	return validation.ValidateManagedSeed(managedSeed)
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
	oldManagedSeed, newManagedSeed := oldObj.(*seedmanagement.ManagedSeed), newObj.(*seedmanagement.ManagedSeed)
	return validation.ValidateManagedSeedUpdate(newManagedSeed, oldManagedSeed)
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

// NewStatusStrategy defines the storage strategy for the status subresource of ManagedSeeds.
func NewStatusStrategy() StatusStrategy {
	return StatusStrategy{NewStrategy()}
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s StatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newManagedSeed := obj.(*seedmanagement.ManagedSeed)
	oldManagedSeed := old.(*seedmanagement.ManagedSeed)
	newManagedSeed.Spec = oldManagedSeed.Spec
}

// ValidateUpdate validates the update on the given old and new object.
func (StatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
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
	return labels.Set(managedSeed.Labels), ToSelectableFields(managedSeed), nil
}

// ToSelectableFields returns a field set that represents the object.
func ToSelectableFields(managedSeed *seedmanagement.ManagedSeed) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	fieldsSet := make(fields.Set, 3)
	fieldsSet[seedmanagement.ManagedSeedShootName] = GetShootName(managedSeed)
	return generic.AddObjectMetaFieldsSet(fieldsSet, &managedSeed.ObjectMeta, true)
}

// ShootNameTriggerFunc returns spec.shoot.name of the given ManagedSeed.
func ShootNameTriggerFunc(obj runtime.Object) string {
	managedSeed, ok := obj.(*seedmanagement.ManagedSeed)
	if !ok {
		return ""
	}

	return GetShootName(managedSeed)
}

// GetShootName returns spec.shoot.name of the given ManagedSeed if it's specified, or an empty string if it's not.
func GetShootName(managedSeed *seedmanagement.ManagedSeed) string {
	if managedSeed.Spec.Shoot == nil {
		return ""
	}
	return managedSeed.Spec.Shoot.Name
}

// SyncSeedBackupCredentials ensures the backup fields
// credentialsRef and secretRef are synced.
// TODO(vpnachev): Remove once the backup.secretRef field is removed.
func SyncSeedBackupCredentials(managedSeed *seedmanagement.ManagedSeed) {
	if managedSeed.Spec.Gardenlet.Config == nil {
		return
	}

	gardenletConfig, ok := managedSeed.Spec.Gardenlet.Config.(*gardenletconfigv1alpha1.GardenletConfiguration)
	if !ok {
		return
	}

	if gardenletConfig.SeedConfig == nil {
		return
	}

	utils.SyncBackupSecretRefAndCredentialsRef(gardenletConfig.SeedConfig.Spec.Backup)
}
