// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

import (
	"github.com/gardener/gardener/pkg/apis/core"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
func ValidateBackupEntrySpecUpdate(newSpec, oldSpec *core.BackupEntrySpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

// ValidateBackupEntryStatusUpdate validates the status field of a BackupEntry object.
func ValidateBackupEntryStatusUpdate(newBackupEntry, oldBackupEntry *core.BackupEntry) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}

func validateBackupEntryName(name string, prefix bool) []string {
	return []string{}
}
