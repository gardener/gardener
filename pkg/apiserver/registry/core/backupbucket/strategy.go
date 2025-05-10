// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
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
	"github.com/gardener/gardener/pkg/apis/core/validation"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type backupBucketStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for BackupBuckets.
var Strategy = backupBucketStrategy{api.Scheme, names.SimpleNameGenerator}

func (backupBucketStrategy) NamespaceScoped() bool {
	return false
}

func (backupBucketStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	backupBucket := obj.(*core.BackupBucket)

	backupBucket.Generation = 1
	backupBucket.Status = core.BackupBucketStatus{}

	SyncBackupSecretRefAndCredentialsRef(&backupBucket.Spec)
}

func (backupBucketStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newBackupBucket := obj.(*core.BackupBucket)
	oldBackupBucket := old.(*core.BackupBucket)
	newBackupBucket.Status = oldBackupBucket.Status

	SyncBackupSecretRefAndCredentialsRef(&newBackupBucket.Spec)

	if mustIncreaseGeneration(oldBackupBucket, newBackupBucket) {
		newBackupBucket.Generation = oldBackupBucket.Generation + 1
	}
}

func mustIncreaseGeneration(oldBackupBucket, newBackupBucket *core.BackupBucket) bool {
	// The BackupBucket specification changes.
	if !apiequality.Semantic.DeepEqual(oldBackupBucket.Spec, newBackupBucket.Spec) {
		return true
	}

	// The deletion timestamp was set.
	if oldBackupBucket.DeletionTimestamp == nil && newBackupBucket.DeletionTimestamp != nil {
		return true
	}

	if kubernetesutils.HasMetaDataAnnotation(&newBackupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile) {
		delete(newBackupBucket.Annotations, v1beta1constants.GardenerOperation)
		return true
	}

	return false
}

func (backupBucketStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	backupBucket := obj.(*core.BackupBucket)
	return validation.ValidateBackupBucket(backupBucket)
}

func (backupBucketStrategy) Canonicalize(_ runtime.Object) {
}

func (backupBucketStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (backupBucketStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldBackupBucket, newBackupBucket := oldObj.(*core.BackupBucket), newObj.(*core.BackupBucket)
	return validation.ValidateBackupBucketUpdate(newBackupBucket, oldBackupBucket)
}

func (backupBucketStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// WarningsOnCreate returns warnings to the client performing a create.
func (backupBucketStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (backupBucketStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

type backupBucketStatusStrategy struct {
	backupBucketStrategy
}

// StatusStrategy defines the storage strategy for the status subresource of BackupBuckets.
var StatusStrategy = backupBucketStatusStrategy{Strategy}

func (backupBucketStatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newBackupBucket := obj.(*core.BackupBucket)
	oldBackupBucket := old.(*core.BackupBucket)
	newBackupBucket.Spec = oldBackupBucket.Spec

	// Ensure credentialsRef is synced even on /status subresources requests.
	// Some clients are patching just the status which still results in update events
	// for those watching the resource.
	SyncBackupSecretRefAndCredentialsRef(&newBackupBucket.Spec)
}

func (backupBucketStatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateBackupBucketStatusUpdate(obj.(*core.BackupBucket), old.(*core.BackupBucket))
}

// ToSelectableFields returns a field set that represents the object
func ToSelectableFields(backupBucket *core.BackupBucket) fields.Set {
	// The purpose of allocation with a given number of elements is to reduce
	// amount of allocations needed to create the fields.Set. If you add any
	// field here or the number of object-meta related fields changes, this should
	// be adjusted.
	backupBucketSpecificFieldsSet := make(fields.Set, 3)
	backupBucketSpecificFieldsSet[core.BackupBucketSeedName] = getSeedName(backupBucket)
	return generic.AddObjectMetaFieldsSet(backupBucketSpecificFieldsSet, &backupBucket.ObjectMeta, true)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	backupBucket, ok := obj.(*core.BackupBucket)
	if !ok {
		return nil, nil, errors.New("not a backupBucket")
	}
	return labels.Set(backupBucket.Labels), ToSelectableFields(backupBucket), nil
}

// MatchBackupBucket returns a generic matcher for a given label and field selector.
func MatchBackupBucket(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:       label,
		Field:       field,
		GetAttrs:    GetAttrs,
		IndexFields: []string{core.BackupBucketSeedName},
	}
}

// SeedNameTriggerFunc returns spec.seedName of given BackupBucket.
func SeedNameTriggerFunc(obj runtime.Object) string {
	backupBucket, ok := obj.(*core.BackupBucket)
	if !ok {
		return ""
	}

	return getSeedName(backupBucket)
}

func getSeedName(backupBucket *core.BackupBucket) string {
	if backupBucket.Spec.SeedName == nil {
		return ""
	}
	return *backupBucket.Spec.SeedName
}

// SyncBackupSecretRefAndCredentialsRef ensures the spec fields
// credentialsRef and secretRef are synced.
// TODO(vpnachev): Remove once the spec.secretRef field is removed.
func SyncBackupSecretRefAndCredentialsRef(backupBucketSpec *core.BackupBucketSpec) {
	emptySecretRef := corev1.SecretReference{}

	// secretRef is set and credentialsRef is not, sync both fields.
	if backupBucketSpec.SecretRef != emptySecretRef && backupBucketSpec.CredentialsRef == nil {
		backupBucketSpec.CredentialsRef = &corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  backupBucketSpec.SecretRef.Namespace,
			Name:       backupBucketSpec.SecretRef.Name,
		}

		return
	}

	// secretRef is unset and credentialsRef refer a secret, sync both fields.
	if backupBucketSpec.SecretRef == emptySecretRef && backupBucketSpec.CredentialsRef != nil &&
		backupBucketSpec.CredentialsRef.APIVersion == "v1" && backupBucketSpec.CredentialsRef.Kind == "Secret" {
		backupBucketSpec.SecretRef = corev1.SecretReference{
			Namespace: backupBucketSpec.CredentialsRef.Namespace,
			Name:      backupBucketSpec.CredentialsRef.Name,
		}

		return
	}

	// in all other cases we can do nothing:
	// - both fields are unset -> we have nothing to sync
	// - both fields are set -> let the validation check if they are correct
	// - credentialsRef refer to WorkloadIdentity -> secretRef should stay unset
}
