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

package controller

import (
	"encoding/json"
	"fmt"

	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
	"github.com/gardener/gardener/pkg/utils"
	"sigs.k8s.io/yaml"
)

func (g Landscaper) computeGardenletChartValues(bootstrapKubeconfig []byte) (map[string]interface{}, error) {
	// do not set the garden bootstrap secret to the chart when the actual bootstrap kubeconfig is nil
	// otherwise this will lead to a chart render error
	if bootstrapKubeconfig == nil && g.gardenletConfiguration.GardenClientConnection != nil {
		g.gardenletConfiguration.GardenClientConnection.BootstrapKubeconfig = nil
	}

	// do not set any parent Gardenlet configuration as the landscaper
	// does not have a parent Gardenlet
	valuesHelper := managedseed.NewValuesHelper(nil, nil)

	// need to use the external version to reuse the values generation of the managed seed
	deploymentV1alpha1 := &seedmanagementv1alpha1.GardenletDeployment{}

	if g.imports.DeploymentConfiguration != nil {
		if err := seedmanagementv1alpha1.Convert_seedmanagement_GardenletDeployment_To_v1alpha1_GardenletDeployment(g.imports.DeploymentConfiguration, deploymentV1alpha1, nil); err != nil {
			return nil, fmt.Errorf("failed to convert GardenletDeployment configuration to internal version for validation: %w", err)
		}
	}

	values, err := valuesHelper.GetGardenletChartValues(deploymentV1alpha1, g.gardenletConfiguration, string(bootstrapKubeconfig))
	if err != nil {
		return nil, err
	}

	if g.imports.ImageVectorOverwrite != nil || g.imports.ComponentImageVectorOverwrites != nil {
		values, err = setImageVectorOverwrites(values, g.imports.ImageVectorOverwrite, g.imports.ComponentImageVectorOverwrites)
		if err != nil {
			return nil, err
		}
	}

	return values, err
}

// setImageVectorOverwrites sets image vector overwrites in the values map
func setImageVectorOverwrites(values map[string]interface{}, imageVectorOverwrite, componentImageVectorOverwrites *json.RawMessage) (map[string]interface{}, error) {
	gardenletValues, err := utils.GetFromValuesMap(values, "global", "gardenlet")
	if err != nil {
		return nil, err
	}

	// Set image vector
	if imageVectorOverwrite != nil {
		yamlImageVector, err := yaml.JSONToYAML(*imageVectorOverwrite)
		if err != nil {
			return nil, fmt.Errorf("failed to convert image vector to yaml: %w", err)
		}
		gardenletValues.(map[string]interface{})["imageVectorOverwrite"] = string(yamlImageVector)
	}

	if componentImageVectorOverwrites != nil {
		yamlImageVector, err := yaml.JSONToYAML(*componentImageVectorOverwrites)
		if err != nil {
			return nil, fmt.Errorf("failed to convert component image vector to yaml: %w", err)
		}
		gardenletValues.(map[string]interface{})["componentImageVectorOverwrites"] = string(yamlImageVector)
	}

	return utils.SetToValuesMap(values, gardenletValues, "global", "gardenlet")
}
