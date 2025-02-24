// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"slices"

	corev1 "k8s.io/api/core/v1"
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

	syncBackupSecretRefAndCredentialsRef(seed.Spec.Backup)
}

// PrepareForUpdate is invoked on update before validation to normalize
// the object.  For example: remove fields that are not to be persisted,
// sort order-insensitive list fields, etc.  This should not remove fields
// whose presence would be considered a validation error.
func (s Strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newSeed := obj.(*core.Seed)
	oldSeed := old.(*core.Seed)
	newSeed.Status = oldSeed.Status

	syncBackupSecretRefAndCredentialsRef(newSeed.Spec.Backup)

	if mustIncreaseGeneration(oldSeed, newSeed) {
		newSeed.Generation = oldSeed.Generation + 1
	}

	syncLegacyAccessRestrictionLabelWithNewFieldOnUpdate(newSeed, oldSeed)
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

// Canonicalize allows an object to be mutated into a canonical form. This
// ensures that code that operates on these objects can rely on the common
// form for things like comparison.  Canonicalize is invoked after
// validation has succeeded but before the object has been persisted.
// This method may mutate the object.
func (Strategy) Canonicalize(obj runtime.Object) {
	seed := obj.(*core.Seed)

	syncLegacyAccessRestrictionLabelWithNewField(seed)
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

// TODO(rfranzke): Remove everything below this line and the legacy access restriction label after
// https://github.com/gardener/dashboard/issues/2120 has been merged and ~6 months have passed to make sure all clients
// have adapted to the new fields in the specifications, and are rolled out.
func syncLegacyAccessRestrictionLabelWithNewField(seed *core.Seed) {
	if seed.Labels["seed.gardener.cloud/eu-access"] == "true" {
		if !slices.ContainsFunc(seed.Spec.AccessRestrictions, func(accessRestriction core.AccessRestriction) bool {
			return accessRestriction.Name == "eu-access-only"
		}) {
			seed.Spec.AccessRestrictions = append(seed.Spec.AccessRestrictions, core.AccessRestriction{Name: "eu-access-only"})
			return
		}
	}

	if slices.ContainsFunc(seed.Spec.AccessRestrictions, func(accessRestriction core.AccessRestriction) bool {
		return accessRestriction.Name == "eu-access-only"
	}) {
		if seed.Labels == nil {
			seed.Labels = make(map[string]string)
		}
		seed.Labels["seed.gardener.cloud/eu-access"] = "true"
	}
}

func syncLegacyAccessRestrictionLabelWithNewFieldOnUpdate(seed, oldSeed *core.Seed) {
	removeAccessRestriction := func(accessRestrictions []core.AccessRestriction, name string) []core.AccessRestriction {
		var updatedAccessRestrictions []core.AccessRestriction
		for _, accessRestriction := range accessRestrictions {
			if accessRestriction.Name != name {
				updatedAccessRestrictions = append(updatedAccessRestrictions, accessRestriction)
			}
		}
		return updatedAccessRestrictions
	}

	if oldSeed.Labels["seed.gardener.cloud/eu-access"] == "true" &&
		seed.Labels["seed.gardener.cloud/eu-access"] != "true" {
		seed.Spec.AccessRestrictions = removeAccessRestriction(seed.Spec.AccessRestrictions, "eu-access-only")
	}
}

func syncBackupSecretRefAndCredentialsRef(backup *core.SeedBackup) {
	if backup == nil {
		return
	}

	emptySecretRef := corev1.SecretReference{}

	// secretRef is set and credentialsRef is not, sync both fields.
	if backup.SecretRef != emptySecretRef && backup.CredentialsRef == nil {
		backup.CredentialsRef = &corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  backup.SecretRef.Namespace,
			Name:       backup.SecretRef.Name,
		}
		return
	}

	// secretRef is unset and credentialsRef refer a secret, sync both fields.
	if backup.SecretRef == emptySecretRef && backup.CredentialsRef != nil &&
		backup.CredentialsRef.APIVersion == "v1" && backup.CredentialsRef.Kind == "Secret" {
		backup.SecretRef = corev1.SecretReference{
			Namespace: backup.CredentialsRef.Namespace,
			Name:      backup.CredentialsRef.Name,
		}

		return
	}

	// in all other cases we can do nothing:
	// - both fields are unset -> we have nothing to sync
	// - both fields are set -> let the validation check if they are correct
	// - credentialsRef refer to WorkloadIdentity -> secretRef should stay unset
}
