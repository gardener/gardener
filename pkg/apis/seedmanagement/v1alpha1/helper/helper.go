// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helper

import (
	"fmt"

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

// ExtractSeedTemplateAndGardenletConfig extracts SeedTemplate and GardenletConfig from the given `managedSeed`.
// An error is returned if either SeedTemplate of GardenletConfig is not specified.
func ExtractSeedTemplateAndGardenletConfig(managedSeed *seedmanagementv1alpha1.ManagedSeed) (*gardencorev1beta1.SeedTemplate, *gardenletv1alpha1.GardenletConfiguration, error) {
	var err error

	gardenlet := managedSeed.Spec.Gardenlet
	if gardenlet == nil {
		return nil, nil, fmt.Errorf("no gardenlet specified in managedseed: %q", managedSeed.Name)
	}

	// Decode gardenlet configuration
	var gardenletConfig *gardenletv1alpha1.GardenletConfiguration
	gardenletConfig, err = encoding.DecodeGardenletConfiguration(&gardenlet.Config, false)
	if err != nil {
		return nil, nil, fmt.Errorf("could not decode gardenlet configuration: %w", err)
	}

	if gardenletConfig.SeedConfig == nil {
		return nil, nil, fmt.Errorf("no seed config found for managedseed %s", managedSeed.Name)
	}

	return &gardenletConfig.SeedConfig.SeedTemplate, gardenletConfig, nil
}
