// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

var (
	// Clean is an alias for `DefaultCleaner().Clean`.
	Clean = DefaultCleaner().Clean

	// CleanAndEnsureGone is an alias for `DefaultCleanOps().CleanAndEnsureGone`.
	CleanAndEnsureGone = DefaultCleanOps().CleanAndEnsureGone
)
