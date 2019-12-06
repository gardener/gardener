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

// ValidateBackupBucket validates a BackupBucket object.
func ValidateBackupBucket(backupBucket *core.BackupBucket) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&backupBucket.ObjectMeta, false, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBackupBucketSpec(&backupBucket.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateBackupBucketUpdate validates a BackupBucket object before an update.
func ValidateBackupBucketUpdate(newBackupBucket, oldBackupBucket *core.BackupBucket) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newBackupBucket.ObjectMeta, &oldBackupBucket.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBackupBucketSpecUpdate(&newBackupBucket.Spec, &oldBackupBucket.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateBackupBucket(newBackupBucket)...)

	return allErrs
}

// ValidateBackupBucketSpec validates the specification of a BackupBucket object.
func ValidateBackupBucketSpec(spec *core.BackupBucketSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.Provider.Type) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("provider.type"), spec.Provider.Type, "type name must not be empty"))
	}
	if len(spec.Provider.Region) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("provider.region"), spec.Provider.Region, "region must not be empty"))
	}

	if spec.SeedName == nil || len(*spec.SeedName) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("seedName"), spec.SeedName, "seed must not be empty"))
	}

	allErrs = append(allErrs, validateSecretReference(spec.SecretRef, fldPath.Child("secretRef"))...)

	return allErrs
}

// ValidateBackupBucketSpecUpdate validates the specification of a BackupBucket object.
func ValidateBackupBucketSpecUpdate(newSpec, oldSpec *core.BackupBucketSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Provider, oldSpec.Provider, fldPath.Child("provider"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.SeedName, oldSpec.SeedName, fldPath.Child("seedName"))...)

	return allErrs
}

// ValidateBackupBucketStatusUpdate validates the status field of a BackupBucket object.
func ValidateBackupBucketStatusUpdate(newBackupBucket, oldBackupBucket *core.BackupBucket) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
