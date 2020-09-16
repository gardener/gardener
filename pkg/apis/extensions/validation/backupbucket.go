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

// ValidateBackupBucket validates a BackupBucket object.
func ValidateBackupBucket(bb *extensionsv1alpha1.BackupBucket) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&bb.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBackupBucketSpec(&bb.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateBackupBucketUpdate validates a BackupBucket object before an update.
func ValidateBackupBucketUpdate(new, old *extensionsv1alpha1.BackupBucket) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&new.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBackupBucketSpecUpdate(&new.Spec, &old.Spec, new.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateBackupBucket(new)...)

	return allErrs
}

// ValidateBackupBucketSpec validates the specification of a BackupBucket object.
func ValidateBackupBucketSpec(spec *extensionsv1alpha1.BackupBucketSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "field is required"))
	}

	if len(spec.Region) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("region"), "field is required"))
	}

	if len(spec.SecretRef.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("secretRef", "name"), "field is required"))
	}

	return allErrs
}

// ValidateBackupBucketSpecUpdate validates the spec of a BackupBucket object before an update.
func ValidateBackupBucketSpecUpdate(new, old *extensionsv1alpha1.BackupBucketSpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(new, old) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, fldPath)...)
		return allErrs
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Type, old.Type, fldPath.Child("type"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Region, old.Region, fldPath.Child("region"))...)

	return allErrs
}

// ValidateBackupBucketStatus validates the status of a BackupBucket object.
func ValidateBackupBucketStatus(spec *extensionsv1alpha1.BackupBucketStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

// ValidateBackupBucketStatusUpdate validates the status field of a BackupBucket object.
func ValidateBackupBucketStatusUpdate(newStatus, oldStatus extensionsv1alpha1.BackupBucketStatus) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
