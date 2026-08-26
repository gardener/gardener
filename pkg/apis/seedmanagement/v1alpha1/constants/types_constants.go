// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	// AnnotationProtectFromDeletion is a constant for an annotation on a replica of a ManagedSeedSet
	// (either ManagedSeed or Shoot) to protect it from deletion..
	AnnotationProtectFromDeletion = "seedmanagement.gardener.cloud/protect-from-deletion"
)
