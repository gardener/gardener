// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

// SetObjectDefaults_NamespacedCloudProfileSpec sets default values for NamespacedCloudProfileSpec objects.
func SetObjectDefaults_NamespacedCloudProfileSpec(in *NamespacedCloudProfileSpec) {
	for i := range in.MachineTypes {
		a := &in.MachineTypes[i]
		SetDefaults_MachineType(a)
	}
	for i := range in.VolumeTypes {
		a := &in.VolumeTypes[i]
		SetDefaults_VolumeType(a)
	}
}

// SetObjectDefaults_NamespacedCloudProfileStatus sets default values for NamespacedCloudProfileStatus objects.
func SetObjectDefaults_NamespacedCloudProfileStatus(_ *NamespacedCloudProfileStatus) {}
