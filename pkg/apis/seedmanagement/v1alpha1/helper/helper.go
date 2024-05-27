// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// GetBootstrap returns the value of the given Bootstrap, or None if nil.
func GetBootstrap(bootstrap *seedmanagementv1alpha1.Bootstrap) seedmanagementv1alpha1.Bootstrap {
	if bootstrap != nil {
		return *bootstrap
	}
	return seedmanagementv1alpha1.BootstrapNone
}

// GardenletConfigFromManagedSeed returns the gardenlet config.
func GardenletConfigFromManagedSeed(gardenlet *seedmanagementv1alpha1.GardenletConfig) *runtime.RawExtension {
	if gardenlet == nil {
		return nil
	}
	return &gardenlet.Config
}

// ExtractSeedTemplateAndGardenletConfig extracts SeedTemplate and GardenletConfig from the given `managedSeed`.
// An error is returned if either SeedTemplate of GardenletConfig is not specified.
func ExtractSeedTemplateAndGardenletConfig(name string, config *runtime.RawExtension) (*gardencorev1beta1.SeedTemplate, *gardenletv1alpha1.GardenletConfiguration, error) {
	var err error

	if config == nil {
		return nil, nil, fmt.Errorf("no gardenlet config provided in object: %q", name)
	}

	// Decode gardenlet configuration
	var gardenletConfig *gardenletv1alpha1.GardenletConfiguration
	gardenletConfig, err = encoding.DecodeGardenletConfiguration(config, false)
	if err != nil {
		return nil, nil, fmt.Errorf("could not decode gardenlet configuration: %w", err)
	}

	if gardenletConfig.SeedConfig == nil {
		return nil, nil, fmt.Errorf("no seed config found for managedseed %s", name)
	}

	return &gardenletConfig.SeedConfig.SeedTemplate, gardenletConfig, nil
}
