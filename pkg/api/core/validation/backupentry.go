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
	allErrs = append(allErrs, ValidateBackupEntrySpec(&backupEntry.Spec, field.NewPath("spec"))...)

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
func ValidateBackupEntrySpec(spec *core.BackupEntrySpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.BucketName) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("bucketName"), spec.BucketName, "bucketName must not be empty"))
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
