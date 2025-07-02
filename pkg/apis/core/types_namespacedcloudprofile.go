// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NamespacedCloudProfile represents certain properties about a provider environment.
type NamespacedCloudProfile struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta

	// Spec defines the provider environment properties.
	Spec NamespacedCloudProfileSpec
	// Most recently observed status of the NamespacedCloudProfile.
	Status NamespacedCloudProfileStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NamespacedCloudProfileList is a collection of NamespacedCloudProfiles.
type NamespacedCloudProfileList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta

	// Items is the list of NamespacedCloudProfiles.
	Items []NamespacedCloudProfile
}

// NamespacedCloudProfileSpec is the specification of a NamespacedCloudProfile.
type NamespacedCloudProfileSpec struct {
	// CABundle is a certificate bundle which will be installed onto every host machine of shoot cluster targeting this profile.
	CABundle *string
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes *KubernetesSettings
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []MachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType
	// Parent contains a reference to a CloudProfile it inherits from.
	Parent CloudProfileReference
	// ProviderConfig contains provider-specific configuration for the profile.
	ProviderConfig *runtime.RawExtension
	// Limits configures operational limits for Shoot clusters using this NamespacedCloudProfile.
	// Any limits specified here override those set in the parent CloudProfile.
	// See https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md.
	Limits *Limits
}

// NamespacedCloudProfileStatus holds the most recently observed status of the NamespacedCloudProfile.
type NamespacedCloudProfileStatus struct {
	// CloudProfileSpec is the most recently generated CloudProfileSpec of the NamespacedCloudProfile.
	CloudProfileSpec CloudProfileSpec
	// ObservedGeneration is the most recent generation observed for this NamespacedCloudProfile.
	ObservedGeneration int64
}

// CloudProfileReference holds the information about a CloudProfile or a NamespacedCloudProfile.
type CloudProfileReference struct {
	// Kind contains a CloudProfile kind.
	Kind string
	// Name contains the name of the referenced CloudProfile.
	Name string
}
