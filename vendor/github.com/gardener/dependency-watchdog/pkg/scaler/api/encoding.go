// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"github.com/ghodss/yaml"
)

// Encode encodes the ProbeDependantsList objects into a string.
func Encode(dependants *ProbeDependantsList) (string, error) {
	data, err := yaml.Marshal(dependants)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Decode decodes the byte stream to ServiceDependants objects.
func Decode(data []byte) (*ProbeDependantsList, error) {
	dependants := new(ProbeDependantsList)
	err := yaml.Unmarshal(data, dependants)
	if err != nil {
		return nil, err
	}
	return dependants, nil
}
