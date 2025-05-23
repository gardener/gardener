// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedmanagement

// Field path constants that are specific to the internal API
// representation.
const (
	// ManagedSeedShootName is the field selector path for finding
	// the Shoot of a seedmanagement.gardener.cloud/v1alpha1 ManagedSeed.
	ManagedSeedShootName = "spec.shoot.name"
)
