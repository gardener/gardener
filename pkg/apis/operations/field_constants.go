// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operations

// Field path constants that are specific to the internal API
// representation.
const (
	// BastionSeedName is the field selector path for finding
	// the Seed cluster of a operations.gardener.cloud/v1alpha1 Bastion.
	BastionSeedName = "spec.seedName"
	// BastionShootName is the field selector path for finding
	// the Shoot name of a operations.gardener.cloud/v1alpha1 Bastion.
	BastionShootName = "spec.shootRef.name"
)
