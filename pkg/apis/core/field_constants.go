// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

// Field path constants that are specific to the internal API
// representation.
const (
	// BackupBucketSeedName is the field selector path for finding
	// the Seed cluster of a core.gardener.cloud/v1beta1 BackupBucket.
	BackupBucketSeedName = "spec.seedName"
	// BackupEntrySeedName is the field selector path for finding
	// the Seed cluster of a core.gardener.cloud/v1beta1 BackupEntry.
	BackupEntrySeedName = "spec.seedName"
	// BackupEntrySeedName is the field selector path for finding
	// the BackupBucket for a core.gardener.cloud/v1beta1 BackupEntry.
	BackupEntryBucketName = "spec.bucketName"

	// InternalSecretType is the field selector path for finding
	// the secret type of a core.gardener.cloud/v1beta1 InternalSecret.
	InternalSecretType = "type"

	// ProjectNamespace is the field selector path for filtering by namespace
	// for core.gardener.cloud/v1beta1 Project.
	ProjectNamespace = "spec.namespace"

	// RegistrationRefName is the field selector path for finding
	// the ControllerRegistration name of a core.gardener.cloud/{v1alpha1,v1beta1} ControllerInstallation.
	RegistrationRefName = "spec.registrationRef.name"
	// SeedRefName is the field selector path for finding
	// the Seed name of a core.gardener.cloud/{v1alpha1,v1beta1} ControllerInstallation.
	SeedRefName = "spec.seedRef.name"

	// ShootCloudProfileName is the field selector path for finding
	// the CloudProfile name of a core.gardener.cloud/{v1alpha1,v1beta1} Shoot.
	ShootCloudProfileName = "spec.cloudProfileName"
	// ShootCloudProfileRefName is the field selector path for finding
	// the referenced CloudProfile name of a core.gardener.cloud/{v1alpha1,v1beta1} Shoot.
	ShootCloudProfileRefName = "spec.cloudProfile.Name"
	// ShootCloudProfileRefKind is the field selector path for finding
	// the referenced CloudProfile kind of a core.gardener.cloud/{v1alpha1,v1beta1} Shoot.
	ShootCloudProfileRefKind = "spec.cloudProfile.Kind"
	// ShootSeedName is the field selector path for finding
	// the Seed cluster of a core.gardener.cloud/{v1alpha1,v1beta1} Shoot.
	ShootSeedName = "spec.seedName"
	// ShootStatusSeedName is the field selector path for finding
	// the Seed cluster of a core.gardener.cloud/{v1alpha1,v1beta1} Shoot
	// referred in the status.
	ShootStatusSeedName = "status.seedName"

	// NamespacedCloudProfileParentRefName is the field selector path for finding
	// the parent CloudProfile of a core.gardener.cloud/v1beta1 NamespacedCloudProfile.
	NamespacedCloudProfileParentRefName = "spec.parent.name"
)
