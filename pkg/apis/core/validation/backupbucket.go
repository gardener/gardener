// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
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

	allErrs = append(allErrs, validateCredentials(spec, fldPath)...)

	return allErrs
}

func validateCredentials(spec *core.BackupBucketSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// How to achieve backward compatibility between secretRef and credentialsRef?
	// - if secretRef is set, credentialsRef must be set and refer the same secret
	// - if secretRef is not set, then credentialsRef must refer a WorkloadIdentity
	//
	// After the sync in the strategy, we can have the following cases:
	// - both secretRef and credentialsRef are unset, which we forbid here
	// - both can be set but refer to different resources, which we forbid here
	// - secretRef can be unset only when workloadIdentity is used, which we respect here

	if spec.CredentialsRef == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("credentialsRef"), "must be set and refer a Secret or WorkloadIdentity"))
	} else {
		allErrs = append(allErrs, ValidateCredentialsRef(*spec.CredentialsRef, fldPath.Child("credentialsRef"))...)

		if spec.CredentialsRef.GroupVersionKind().String() == corev1.SchemeGroupVersion.WithKind("Secret").String() {
			if spec.SecretRef.Namespace != spec.CredentialsRef.Namespace || spec.SecretRef.Name != spec.CredentialsRef.Name {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("secretRef"), "must refer to the same secret as `spec.credentialsRef`"))
			}
		} else if (spec.SecretRef != corev1.SecretReference{}) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("secretRef"), "must not be set when `spec.credentialsRef` refer to resource other than secret"))
		}

		// TODO(vpnachev): Allow WorkloadIdentities once the support in the controllers and components is fully implemented.
		if spec.CredentialsRef.APIVersion == securityv1alpha1.SchemeGroupVersion.String() &&
			spec.CredentialsRef.Kind == "WorkloadIdentity" {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("credentialsRef"), "support for WorkloadIdentity as backup credentials is not yet fully implemented"))
		}
	}

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
func ValidateBackupBucketStatusUpdate(_, _ *core.BackupBucket) field.ErrorList {
	allErrs := field.ErrorList{}

	return allErrs
}
