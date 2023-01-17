// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1

import (
	"k8s.io/utils/pointer"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// SetDefaults_MachineImageVersion sets default values for MachineImageVersion objects.
func SetDefaults_MachineImageVersion(obj *MachineImageVersion) {
	if len(obj.CRI) == 0 {
		obj.CRI = []CRI{
			{
				Name: CRINameDocker,
			},
		}
	}

	if len(obj.Architectures) == 0 {
		obj.Architectures = []string{v1beta1constants.ArchitectureAMD64}
	}
}

// SetDefaults_MachineType sets default values for MachineType objects.
func SetDefaults_MachineType(obj *MachineType) {
	if obj.Architecture == nil {
		obj.Architecture = pointer.String(v1beta1constants.ArchitectureAMD64)
	}

	if obj.Usable == nil {
		obj.Usable = pointer.Bool(true)
	}
}

// SetDefaults_VolumeType sets default values for VolumeType objects.
func SetDefaults_VolumeType(obj *VolumeType) {
	if obj.Usable == nil {
		obj.Usable = pointer.Bool(true)
	}
}
