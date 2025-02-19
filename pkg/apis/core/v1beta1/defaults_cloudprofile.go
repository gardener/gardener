// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"
)

// SetDefaults_MachineImage sets default values for MachineImage objects.
func SetDefaults_MachineImage(obj *MachineImage) {
	if obj.UpdateStrategy == nil {
		updateStrategyMajor := UpdateStrategyMajor
		obj.UpdateStrategy = &updateStrategyMajor
	}
}

// SetDefaults_MachineImageVersion sets default values for MachineImageVersion objects.
func SetDefaults_MachineImageVersion(obj *MachineImageVersion) {
	if len(obj.CRI) == 0 {
		obj.CRI = []CRI{
			{
				Name: CRINameContainerD,
			},
		}
	}
}

// SetDefaults_MachineType sets default values for MachineType objects.
func SetDefaults_MachineType(obj *MachineType) {
	if obj.Usable == nil {
		obj.Usable = ptr.To(true)
	}
}

// SetDefaults_VolumeType sets default values for VolumeType objects.
func SetDefaults_VolumeType(obj *VolumeType) {
	if obj.Usable == nil {
		obj.Usable = ptr.To(true)
	}
}

// GetV1JsonCapabilities transforms the given keys and values into a JSON-string and returns it as v1.JSON object.
// The keys and values must have the same length.
func GetV1JsonCapabilities(keys []string, values []string) v1.JSON {
	// Example:
	//v1.JSON{Raw: []byte(`{"` +
	//keys[0] + `":"` + values[0] + `,` +
	//keys[1] + `":"` + value[1] +
	//`"}`)}

	if len(keys) != len(values) {
		panic("keys and values must have the same length")
	}
	if len(keys) == 0 {
		return v1.JSON{Raw: []byte(`{}`)}
	}
	var capabilities v1.JSON
	jsonString := "{"
	for i := 0; i < len(keys); i++ {
		jsonString += `"` + keys[i] + `":"` + values[i] + `"`
		if i < len(keys)-1 {
			jsonString += ","
		}
	}
	jsonString += "}"
	capabilities.Raw = []byte(jsonString)
	return capabilities
}
