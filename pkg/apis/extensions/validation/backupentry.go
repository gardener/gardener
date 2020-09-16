// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateBackupEntry validates a BackupEntry object.
func ValidateBackupEntry(be *extensionsv1alpha1.BackupEntry) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&be.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBackupEntrySpec(&be.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateBackupEntryUpdate validates a BackupEntry object before an update.
func ValidateBackupEntryUpdate(new, old *extensionsv1alpha1.BackupEntry) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBackupEntrySpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateBackupEntry(new)...)

	return allErrs
}

// ValidateBackupEntrySpec validates the specification of a BackupEntry object.
func ValidateBackupEntrySpec(spec *extensionsv1alpha1.BackupEntrySpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field is required"))
	}

	if len(spec.BucketName) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("bucketName"), "field is required"))
	}

	if len(spec.Region) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("region"), "field is required"))
	}

	if len(spec.SecretRef.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("secretRef", "name"), "field is required"))
	}

	return allErrs
}

// ValidateBackupEntrySpecUpdate validates the spec of a BackupEntry object before an update.
func ValidateBackupEntrySpecUpdate(new, old *extensionsv1alpha1.BackupEntrySpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, fldPath)...)
		return allErrs
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Region, old.Region, fldPath.Child("region"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.BucketName, old.BucketName, fldPath.Child("bucketName"))...)

	return allErrs
}

// ValidateBackupEntryStatus validates the status of a BackupEntry object.
func ValidateBackupEntryStatus(spec *extensionsv1alpha1.BackupEntryStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

// ValidateBackupEntryStatusUpdate validates the status field of a BackupEntry object.
func ValidateBackupEntryStatusUpdate(newStatus, oldStatus extensionsv1alpha1.BackupEntryStatus) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
