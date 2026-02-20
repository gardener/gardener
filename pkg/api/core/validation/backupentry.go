// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
)

// ValidateBackupEntry validates a BackupEntry object.
func ValidateBackupEntry(backupEntry *core.BackupEntry) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&backupEntry.ObjectMeta, true, validateBackupEntryName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBackupEntrySpec(&backupEntry.Spec, backupEntry.Namespace, field.NewPath("spec"))...)

	return allErrs
}

// ValidateBackupEntryUpdate validates a BackupEntry object before an update.
func ValidateBackupEntryUpdate(newBackupEntry, oldBackupEntry *core.BackupEntry) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newBackupEntry.ObjectMeta, &oldBackupEntry.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBackupEntrySpecUpdate(&newBackupEntry.Spec, &oldBackupEntry.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateBackupEntry(newBackupEntry)...)

	return allErrs
}

// ValidateBackupEntrySpec validates the specification of a BackupEntry object.
func ValidateBackupEntrySpec(spec *core.BackupEntrySpec, namespace string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.BucketName) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("bucketName"), spec.BucketName, "bucketName must not be empty"))
	}

	if spec.SeedName == nil && spec.ShootRef == nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("seedName"), spec.SeedName, "either .spec.seedName or .spec.shootRef must be set"))
	} else if spec.SeedName != nil && spec.ShootRef != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("seedName"), spec.SeedName, "not both .spec.seedName and .spec.shootRef can be set at the same time"))
	} else if spec.SeedName != nil && len(*spec.SeedName) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("seedName"), spec.SeedName, "seed must not be empty"))
	} else if spec.ShootRef != nil {
		if expectedAPIVersion := "core.gardener.cloud/v1beta1"; spec.ShootRef.APIVersion != expectedAPIVersion {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("shootRef").Child("apiVersion"), "must be "+expectedAPIVersion))
		}
		if expectedKind := "Shoot"; spec.ShootRef.Kind != expectedKind {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("shootRef").Child("kind"), "must be "+expectedKind))
		}
		if len(spec.ShootRef.Name) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("shootRef").Child("name"), spec.ShootRef.Name, "name must not be empty"))
		}
		if len(spec.ShootRef.Namespace) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("shootRef").Child("namespace"), spec.ShootRef.Namespace, "namespace must not be empty"))
		} else if spec.ShootRef.Namespace != namespace {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("shootRef").Child("namespace"), "cannot reference a Shoot from another namespace"))
		}
	}

	return allErrs
}

// ValidateBackupEntrySpecUpdate validates the specification of a BackupEntry object.
func ValidateBackupEntrySpecUpdate(_, _ *core.BackupEntrySpec, _ *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

// ValidateBackupEntryStatusUpdate validates the status field of a BackupEntry object.
func ValidateBackupEntryStatusUpdate(_, _ *core.BackupEntry) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

func validateBackupEntryName(_ string, _ bool) []string {
	return []string{}
}
