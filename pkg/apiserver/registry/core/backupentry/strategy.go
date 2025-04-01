// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/core/validation"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

type backupEntryStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// NewStrategy returns a new storage strategy for BackupEntries.
func NewStrategy() backupEntryStrategy {
	return backupEntryStrategy{api.Scheme, names.SimpleNameGenerator}
}

// Strategy defines the storage strategy for BackupEntries.
var Strategy = backupEntryStrategy{api.Scheme, names.SimpleNameGenerator}

func (backupEntryStrategy) NamespaceScoped() bool {
	return true
}

func (backupEntryStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	backupEntry := obj.(*core.BackupEntry)

	backupEntry.Generation = 1
	backupEntry.Status = core.BackupEntryStatus{}
}

func (backupEntryStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newBackupEntry := obj.(*core.BackupEntry)
	oldBackupEntry := old.(*core.BackupEntry)
	newBackupEntry.Status = oldBackupEntry.Status

	if mustIncreaseGeneration(oldBackupEntry, newBackupEntry) {
		newBackupEntry.Generation = oldBackupEntry.Generation + 1
	}
}

func mustIncreaseGeneration(oldBackupEntry, newBackupEntry *core.BackupEntry) bool {
	// The BackupEntry specification changes.
	if !apiequality.Semantic.DeepEqual(oldBackupEntry.Spec, newBackupEntry.Spec) {
		return true
	}

	// The deletion timestamp was set.
	if oldBackupEntry.DeletionTimestamp == nil && newBackupEntry.DeletionTimestamp != nil {
		return true
	}

	oldPresent, _ := strconv.ParseBool(oldBackupEntry.Annotations[core.BackupEntryForceDeletion])
	newPresent, _ := strconv.ParseBool(newBackupEntry.Annotations[core.BackupEntryForceDeletion])
	if oldPresent != newPresent && newPresent {
		return true
	}

	if v1beta1helper.HasOperationAnnotation(newBackupEntry.Annotations) {
		// Remove the operation annotation if its value is not "restore"
		// If it's "restore", it will be removed at the end of the reconciliation since it's needed
		// to properly determine that the operation is "restore, and not "reconcile"
		if newBackupEntry.Annotations[v1beta1constants.GardenerOperation] != v1beta1constants.GardenerOperationRestore {
			delete(newBackupEntry.Annotations, v1beta1constants.GardenerOperation)
		} else {
			// we don't want to cause duplicate reconciliations because this annotation is removed only at the end of operation
			if oldBackupEntry.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore {
				return false
			}
		}
		return true
	}

	return false
}

func (backupEntryStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	backupEntry := obj.(*core.BackupEntry)
	return validation.ValidateBackupEntry(backupEntry)
}

func (backupEntryStrategy) Canonicalize(obj runtime.Object) {
	backupEntry := obj.(*core.BackupEntry)

	gardenerutils.MaintainSeedNameLabels(backupEntry, backupEntry.Spec.SeedName, backupEntry.Status.SeedName)
}

func (backupEntryStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (backupEntryStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldBackupEntry, newBackupEntry := oldObj.(*core.BackupEntry), newObj.(*core.BackupEntry)
	return validation.ValidateBackupEntryUpdate(newBackupEntry, oldBackupEntry)
}

func (backupEntryStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings to the client performing a create.
func (backupEntryStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (backupEntryStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

type backupEntryStatusStrategy struct {
	backupEntryStrategy
}

// StatusStrategy defines the storage strategy for the status subresource of BackupEntries.
var StatusStrategy = backupEntryStatusStrategy{Strategy}

func (backupEntryStatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newBackupEntry := obj.(*core.BackupEntry)
	oldBackupEntry := old.(*core.BackupEntry)
	newBackupEntry.Spec = oldBackupEntry.Spec
}

func (backupEntryStatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateBackupEntryStatusUpdate(obj.(*core.BackupEntry), old.(*core.BackupEntry))
}

// ToSelectableFields returns a field set that represents the object
func ToSelectableFields(backupEntry *core.BackupEntry) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	backupEntrySpecificFieldsSet := make(fields.Set, 4)
	backupEntrySpecificFieldsSet[core.BackupEntrySeedName] = getSeedName(backupEntry)
	backupEntrySpecificFieldsSet[core.BackupEntryBucketName] = backupEntry.Spec.BucketName
	return generic.AddObjectMetaFieldsSet(backupEntrySpecificFieldsSet, &backupEntry.ObjectMeta, true)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	backupEntry, ok := obj.(*core.BackupEntry)
	if !ok {
		return nil, nil, errors.New("not a backupEntry")
	}
	return backupEntry.Labels, ToSelectableFields(backupEntry), nil
}

// MatchBackupEntry returns a generic matcher for a given label and field selector.
func MatchBackupEntry(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:       label,
		Field:       field,
		GetAttrs:    GetAttrs,
		IndexFields: []string{core.BackupEntrySeedName, core.BackupEntryBucketName},
	}
}

// SeedNameIndexFunc returns spec.seedName of given BackupEntry.
func SeedNameIndexFunc(obj any) ([]string, error) {
	backupEntry, ok := obj.(*core.BackupEntry)
	if !ok {
		return nil, fmt.Errorf("expected *core.BackupEntry but got %T", obj)
	}

	return []string{getSeedName(backupEntry)}, nil
}

func getSeedName(backupEntry *core.BackupEntry) string {
	if backupEntry.Spec.SeedName == nil {
		return ""
	}
	return *backupEntry.Spec.SeedName
}

// BucketNameIndexFunc returns spec.BucketName of given BackupEntry.
func BucketNameIndexFunc(obj any) ([]string, error) {
	backupEntry, ok := obj.(*core.BackupEntry)
	if !ok {
		return nil, fmt.Errorf("expected *core.BackupEntry but got %T", obj)
	}

	return []string{backupEntry.Spec.BucketName}, nil
}
