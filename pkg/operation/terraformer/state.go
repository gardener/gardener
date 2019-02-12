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
	"context"
	"encoding/json"
	"fmt"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

type terraformState struct {
	Modules []struct {
		Outputs map[string]map[string]interface{} `json:"outputs"`
	} `json:"modules"`
}

// GetState returns the Terraform state as byte slice.
func (t *Terraformer) GetState() ([]byte, error) {
	ctx := context.TODO()
	configMap := &corev1.ConfigMap{}
	if err := t.client.Get(ctx, kutil.Key(t.namespace, t.stateName), configMap); err != nil {
		return nil, err
	}

	return []byte(configMap.Data[StateKey]), nil
}

// GetStateOutputVariables returns the given <variable> from the given Terraform <stateData>.
// In case the variable was not found, an error is returned.
func (t *Terraformer) GetStateOutputVariables(variables ...string) (map[string]string, error) {
	var (
		state  terraformState
		output = make(map[string]string)

		wantedVariables = sets.NewString(variables...)
		foundVariables  = sets.NewString()
	)

	stateConfigMap, err := t.GetState()
	if err != nil {
		return nil, err
	}

	if len(stateConfigMap) == 0 {
		return nil, &variablesNotFoundError{wantedVariables.List()}
	}

	if err := json.Unmarshal(stateConfigMap, &state); err != nil {
		return nil, err
	}

	for _, variable := range variables {
		if value, ok := state.Modules[0].Outputs[variable]["value"]; ok {
			output[variable] = value.(string)
			foundVariables.Insert(variable)
		}
	}

	if wantedVariables.Len() != foundVariables.Len() {
		return nil, &variablesNotFoundError{wantedVariables.Difference(foundVariables).List()}
	}

	return output, nil
}

// isStateEmpty returns true if the Terraform state is empty, and false otherwise.
func (t *Terraformer) isStateEmpty() bool {
	state, err := t.GetState()
	if err != nil {
		if apierrors.IsNotFound(err) {
			return true
		}
		return false
	}
	return len(state) == 0
}

type variablesNotFoundError struct {
	variables []string
}

// Error prints the error message of the variablesNotFound error.
func (e *variablesNotFoundError) Error() string {
	return fmt.Sprintf("could not find all requested variables: %+v", e.variables)
}

// IsVariablesNotFoundError returns true if the error indicates that not all variables have been found.
func IsVariablesNotFoundError(err error) bool {
	switch err.(type) {
	case *variablesNotFoundError:
		return true
	}
	return false
}
