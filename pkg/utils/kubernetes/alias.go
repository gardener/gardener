// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

var (
	// WaitUntilStatefulSetScaled is an alias for WaitUntilStatefulSetScaledToDesiredReplicas. Exposed for testing.
	WaitUntilStatefulSetScaled = WaitUntilStatefulSetScaledToDesiredReplicas
)
