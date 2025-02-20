// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

// SetobjectdefaultsNamespacedcloudprofilespec sets default values for NamespacedCloudProfileSpec objects.
func SetobjectdefaultsNamespacedcloudprofilespec(in *NamespacedCloudProfileSpec) {
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
