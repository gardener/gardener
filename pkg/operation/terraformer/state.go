// Copyright 2018 The Gardener Authors.
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

package terraformer

import (
	"errors"

	"github.com/gardener/gardener/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// GetState returns the Terraform state as byte slice.
func (t *Terraformer) GetState() ([]byte, error) {
	configmap, err := t.K8sSeedClient.GetConfigMap(t.Namespace, t.StateName)
	if err != nil {
		return nil, err
	}
	return []byte(configmap.Data["terraform.tfstate"]), nil
}

// GetStateOutputVariables returns the given <variable> from the given Terraform <stateData>.
// In case the variable was not found, an error is returned.
func (t *Terraformer) GetStateOutputVariables(variables ...string) (map[string]string, error) {
	stateConfigMap, err := t.GetState()
	if err != nil {
		return nil, err
	}
	state := utils.ConvertJSONToMap(stateConfigMap)

	output := make(map[string]string)
	for _, variable := range variables {
		value, err := state.String("modules", "0", "outputs", variable, "value")
		if err != nil {
			return nil, err
		}
		output[variable] = value
	}

	if len(output) != len(variables) {
		return nil, errors.New("could not find all requested variables")
	}

	return output, nil
}

// IsStateEmpty returns true if the Terraform state is empty, and false otherwise.
func (t *Terraformer) IsStateEmpty() bool {
	state, err := t.GetState()
	if err != nil {
		if apierrors.IsNotFound(err) {
			return true
		}
		return false
	}

	return len(state) == 0
}
