// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NamespacedCloudProfile represents certain properties about a provider environment.
type NamespacedCloudProfile struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Spec defines the provider environment properties.
	Spec NamespacedCloudProfileSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// Most recently observed status of the NamespacedCloudProfile.
	Status NamespacedCloudProfileStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NamespacedCloudProfileList is a collection of NamespacedCloudProfiles.
type NamespacedCloudProfileList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of NamespacedCloudProfiles.
	Items []NamespacedCloudProfile `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// NamespacedCloudProfileSpec is the specification of a NamespacedCloudProfile.
type NamespacedCloudProfileSpec struct {
	// CABundle is a certificate bundle which will be installed onto every host machine of shoot cluster targeting this profile.
	// +optional
	CABundle *string `json:"caBundle,omitempty" protobuf:"bytes,1,opt,name=caBundle"`
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	// +optional
	Kubernetes *KubernetesSettings `json:"kubernetes,omitempty" protobuf:"bytes,2,opt,name=kubernetes"`
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	MachineImages []MachineImage `json:"machineImages,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,3,opt,name=machineImages"`
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	MachineTypes []MachineType `json:"machineTypes,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,4,opt,name=machineTypes"`
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	VolumeTypes []VolumeType `json:"volumeTypes,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,6,opt,name=volumeTypes"`
	// Parent contains a reference to a CloudProfile it inherits from.
	Parent CloudProfileReference `json:"parent" protobuf:"bytes,7,req,name=parent"`
	// ProviderConfig contains provider-specific configuration for the profile.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,8,opt,name=providerConfig"`
	// Limits configures operational limits for Shoot clusters using this NamespacedCloudProfile.
	// If a limit is already set in the parent CloudProfile, it can only be more restrictive in the NamespacedCloudProfile.
	// See https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md.
	// +optional
	Limits *Limits `json:"limits,omitempty" protobuf:"bytes,9,opt,name=limits"`
}

// NamespacedCloudProfileStatus holds the most recently observed status of the NamespacedCloudProfile.
type NamespacedCloudProfileStatus struct {
	// CloudProfile is the most recently generated CloudProfile of the NamespacedCloudProfile.
	CloudProfileSpec CloudProfileSpec `json:"cloudProfileSpec,omitempty" protobuf:"bytes,1,req,name=cloudProfileSpec"`
	// ObservedGeneration is the most recent generation observed for this NamespacedCloudProfile.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,2,opt,name=observedGeneration"`
}

// CloudProfileReference holds the information about a CloudProfile or a NamespacedCloudProfile.
type CloudProfileReference struct {
	// Kind contains a CloudProfile kind.
	Kind string `json:"kind" protobuf:"bytes,1,req,name=kind"`
	// Name contains the name of the referenced CloudProfile.
	Name string `json:"name" protobuf:"bytes,2,req,name=name"`
}
