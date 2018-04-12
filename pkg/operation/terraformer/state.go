// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"encoding/json"
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type terraformState struct {
	Modules []struct {
		Outputs map[string]map[string]interface{} `json:"outputs"`
	} `json:"modules"`
}

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
	var (
		state  terraformState
		output = make(map[string]string)
	)

	stateConfigMap, err := t.GetState()
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(stateConfigMap, &state); err != nil {
		return nil, err
	}

	for _, variable := range variables {
		if value, ok := state.Modules[0].Outputs[variable]["value"]; ok {
			output[variable] = value.(string)
		}
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
