// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
