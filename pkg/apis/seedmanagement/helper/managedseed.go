// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// ExtractSeedSpec extracts the seed spec from the ManagedSeed.
func ExtractSeedSpec(managedSeed *seedmanagement.ManagedSeed) (*gardencore.SeedSpec, error) {
	gardenlet := managedSeed.Spec.Gardenlet
	if gardenlet.Config == nil {
		return nil, fmt.Errorf("no gardenlet config specified in managedseed %s", managedSeed.Name)
	}

	gardenletConfig, ok := gardenlet.Config.(*gardenletconfigv1alpha1.GardenletConfiguration)
	if !ok {
		return nil, fmt.Errorf("expected *gardenletconfigv1alpha1.GardenletConfiguration but got %T", gardenlet.Config)
	}
	if gardenletConfig.SeedConfig == nil {
		return nil, fmt.Errorf("no seed config found for managedseed %s", managedSeed.Name)
	}

	seedTemplate, err := gardencorehelper.ConvertSeedTemplate(&gardenletConfig.SeedConfig.SeedTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to convert SeedTemplate of ManagedSeed %s: %w", managedSeed.Name, err)
	}

	return &seedTemplate.Spec, nil
}
