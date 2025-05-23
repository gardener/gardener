// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dependencywatchdog

import (
	proberapi "github.com/gardener/dependency-watchdog/api/prober"
	weederapi "github.com/gardener/dependency-watchdog/api/weeder"
)

type (
	// WeederConfigurationFunc is a function alias for returning configuration for the dependency-watchdog
	// (weeder role).
	WeederConfigurationFunc func() (map[string]weederapi.DependantSelectors, error)

	// ProberConfigurationFunc is a function alias for returning configuration for the dependency-watchdog
	// (prober role).
	ProberConfigurationFunc func() ([]proberapi.DependentResourceInfo, error)
)
