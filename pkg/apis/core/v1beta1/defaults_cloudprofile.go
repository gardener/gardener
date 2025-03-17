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
	// If CapabilitiesDefinition is defined no defaulting for Architecture is required
	// as the default is defined in the CloudProfile itself in Spec.CapabilitiesDefinition.architecture
	if cloudProfile.Spec.CapabilitiesDefinition.HasEntries() {
		return
	}

	for i := range cloudProfile.Spec.MachineImages {
		machineImage := &cloudProfile.Spec.MachineImages[i]

		for j := range machineImage.Versions {
			b := &machineImage.Versions[j]
			if len(b.Architectures) == 0 {
				b.Architectures = []string{v1beta1constants.ArchitectureAMD64}
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
