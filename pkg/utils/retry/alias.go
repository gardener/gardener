// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package retry

var (
	// Until is an alias for `DefaultOps().Until`.
	Until = DefaultOps().Until

	// UntilTimeout is an alias for `DefaultOps().New`.
	UntilTimeout = DefaultOps().UntilTimeout

	// Interval is an alias for `DefaultIntervalFactory().New`.
	Interval = DefaultIntervalFactory().New
)
