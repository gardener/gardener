// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
)

// ExtractSeedSpec extracts the seed spec from the ManagedSeed.
func ExtractSeedSpec(managedSeed *seedmanagement.ManagedSeed) (*gardencore.SeedSpec, error) {
	gardenlet := managedSeed.Spec.Gardenlet
	if gardenlet.Config == nil {
		return nil, fmt.Errorf("no gardenlet config specified in managedseed %s", managedSeed.Name)
	}

	gardenletConfig, err := gardenlethelper.ConvertGardenletConfiguration(gardenlet.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to convert gardenlet config for managedseed %s: %w", managedSeed.Name, err)
	}

	return &gardenletConfig.SeedConfig.Spec, nil
}
