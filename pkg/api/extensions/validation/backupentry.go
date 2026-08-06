// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	"github.com/go-test/deep"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ValidateBackupEntry validates a BackupEntry object.
func ValidateBackupEntry(be *extensionsv1alpha1.BackupEntry) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&be.ObjectMeta, false, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBackupEntrySpec(&be.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateBackupEntryUpdate validates a BackupEntry object before an update.
func ValidateBackupEntryUpdate(newBackupEntry, oldBackupEntry *extensionsv1alpha1.BackupEntry) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newBackupEntry.ObjectMeta, &oldBackupEntry.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBackupEntrySpecUpdate(&newBackupEntry.Spec, &oldBackupEntry.Spec, newBackupEntry.DeletionTimestamp != nil, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateBackupEntry(newBackupEntry)...)

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
func ValidateBackupEntrySpecUpdate(newSpec, oldSpec *extensionsv1alpha1.BackupEntrySpec, deletionTimestampSet bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deletionTimestampSet && !apiequality.Semantic.DeepEqual(newSpec, oldSpec) {
		diff := deep.Equal(newSpec, oldSpec)
		return field.ErrorList{field.Forbidden(fldPath, fmt.Sprintf("cannot update backup entry spec if deletion timestamp is set. Requested changes: %s", strings.Join(diff, ",")))}
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Type, oldSpec.Type, fldPath.Child("type"))...)

	return allErrs
}
