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
