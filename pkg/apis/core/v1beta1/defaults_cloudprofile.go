// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// SetDefaults_CloudProfile sets default values for CloudProfile objects.
func SetDefaults_CloudProfile(cloudProfile *CloudProfile) {
	// If Capabilities are defined, no Architecture field defaults are required.
	// The default value is instead defined by the value(s) defined in cloudProfile.Spec.Capabilities.architecture.
	if len(cloudProfile.Spec.Capabilities) > 0 {
		return
	}

	for imageIdx := range cloudProfile.Spec.MachineImages {
		machineImage := &cloudProfile.Spec.MachineImages[imageIdx]

		for versionIdx := range machineImage.Versions {
			imageVersion := &machineImage.Versions[versionIdx]
			if len(imageVersion.Architectures) == 0 {
				imageVersion.Architectures = []string{v1beta1constants.ArchitectureAMD64}
			}
		}
	}
	for i := range cloudProfile.Spec.MachineTypes {
		machineType := &cloudProfile.Spec.MachineTypes[i]
		if machineType.Architecture == nil {
			machineType.Architecture = ptr.To(v1beta1constants.ArchitectureAMD64)
		}
	}
}

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
