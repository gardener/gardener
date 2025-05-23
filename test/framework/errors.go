// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"errors"
)

var (
	// ErrNoRepositoriesFound no repositories found in repository file
	ErrNoRepositoriesFound = errors.New("no repositories found in repository file")

	// ErrNoInternalIPsForNodeWasFound no internal IPs were found for node
	ErrNoInternalIPsForNodeWasFound = errors.New("no internal IPs were found for node")

	// ErrNoRunningPodsFound no running pods were found
	ErrNoRunningPodsFound = errors.New("no running pods were found")
)
