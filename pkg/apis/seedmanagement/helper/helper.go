// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strconv"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// GetBootstrap returns the value of the given Bootstrap, or None if nil.
func GetBootstrap(bootstrap *seedmanagement.Bootstrap) seedmanagement.Bootstrap {
	if bootstrap != nil {
		return *bootstrap
	}
	return seedmanagement.BootstrapNone
}

// IsMultiZonalManagedSeed checks if a managed seed is multi-zonal.
func IsMultiZonalManagedSeed(managedSeed *seedmanagement.ManagedSeed) (bool, error) {
	seedTemplate, _, err := ExtractSeedTemplate(managedSeed)
	if err != nil {
		return false, err
	}
	if multiZonalLabelVal, ok := seedTemplate.ObjectMeta.Labels[v1beta1constants.LabelSeedMultiZonal]; ok {
		if len(multiZonalLabelVal) == 0 {
			return true, nil
		}
		// There is no need to check any error here as the value has already been validated as part of API validation. If the control has come here then value is a proper boolean value.
		val, _ := strconv.ParseBool(multiZonalLabelVal)
		return val, nil
	}
	return seedTemplate.Spec.HighAvailability != nil && seedTemplate.Spec.HighAvailability.FailureTolerance.Type == gardencore.FailureToleranceTypeZone, nil
}

// ExtractSeedTemplate extracts the seed template from ManagedSeed along with the path for the SeedTemplate.
func ExtractSeedTemplate(managedSeed *seedmanagement.ManagedSeed) (*gardencore.SeedTemplate, *field.Path, error) {
	if managedSeed.Spec.SeedTemplate != nil {
		return managedSeed.Spec.SeedTemplate, field.NewPath("spec", "seedTemplate"), nil
	}
	gardenletConfig, err := getGardenletConfiguration(managedSeed)
	if err != nil {
		return nil, nil, err
	}
	if gardenletConfig != nil && gardenletConfig.SeedConfig != nil {
		return &gardenletConfig.SeedConfig.SeedTemplate, field.NewPath("spec", "gardenlet", "config", "seedConfig"), nil
	}
	return nil, nil, fmt.Errorf("no seed template found for managedseed %s", managedSeed.Name)
}

// getGardenletConfiguration converts and gets the config.GardenletConfiguration from the ManagedSeed if one is defined.
func getGardenletConfiguration(managedSeed *seedmanagement.ManagedSeed) (*config.GardenletConfiguration, error) {
	gardenlet := managedSeed.Spec.Gardenlet
	if gardenlet == nil || gardenlet.Config == nil {
		return nil, nil
	}
	gardenletConfig, err := confighelper.ConvertGardenletConfiguration(gardenlet.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to convert gardenlet config for managedseed %s: %w", managedSeed.Name, err)
	}
	return gardenletConfig, nil
}
