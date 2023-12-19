// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
)

// GetBootstrap returns the value of the given Bootstrap, or None if nil.
func GetBootstrap(bootstrap *seedmanagement.Bootstrap) seedmanagement.Bootstrap {
	if bootstrap != nil {
		return *bootstrap
	}
	return seedmanagement.BootstrapNone
}

// ExtractSeedSpec extracts the seed spec from the ManagedSeed.
func ExtractSeedSpec(managedSeed *seedmanagement.ManagedSeed) (*gardencore.SeedSpec, error) {
	gardenlet := managedSeed.Spec.Gardenlet
	if gardenlet == nil || gardenlet.Config == nil {
		return nil, fmt.Errorf("no gardenlet config specified in managedseed %s", managedSeed.Name)
	}

	gardenletConfig, err := gardenlethelper.ConvertGardenletConfiguration(gardenlet.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to convert gardenlet config for managedseed %s: %w", managedSeed.Name, err)
	}
	if gardenletConfig.SeedConfig == nil {
		return nil, fmt.Errorf("no seed config found for managedseed %s", managedSeed.Name)
	}

	return &gardenletConfig.SeedConfig.Spec, nil
}
