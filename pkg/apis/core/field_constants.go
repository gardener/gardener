// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
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

	// RegistrationRefName is the field selector path for finding
	// the ControllerRegistration name of a core.gardener.cloud/{v1beta1,v1beta1} ControllerInstallation.
	RegistrationRefName = "spec.registrationRef.name"
	// SeedRefName is the field selector path for finding
	// the Seed name of a core.gardener.cloud/{v1beta1,v1beta1} ControllerInstallation.
	SeedRefName = "spec.seedRef.name"

	// ShootCloudProfileName is the field selector path for finding
	// the CloudProfile name of a core.gardener.cloud/{v1alpha1,v1beta1} Shoot.
	ShootCloudProfileName = "spec.cloudProfileName"
	// ShootSeedName is the field selector path for finding
	// the Seed cluster of a core.gardener.cloud/{v1alpha1,v1beta1} Shoot.
	ShootSeedName = "spec.seedName"
	// ShootStatusSeedName is the field selector path for finding
	// the Seed cluster of a core.gardener.cloud/{v1alpha1,v1beta1} Shoot
	// referred in the status.
	ShootStatusSeedName = "status.seedName"
)
