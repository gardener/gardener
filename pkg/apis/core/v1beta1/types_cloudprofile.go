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

package v1beta1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfile represents certain properties about a provider environment.
type CloudProfile struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec defines the provider environment properties.
	// +optional
	Spec CloudProfileSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfileList is a collection of CloudProfiles.
type CloudProfileList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of CloudProfiles.
	Items []CloudProfile `json:"items"`
}

// CloudProfileSpec is the specification of a CloudProfile.
// It must contain exactly one of its defined keys.
type CloudProfileSpec struct {
	// CABundle is a certificate bundle which will be installed onto every host machine of shoot cluster targeting this profile.
	// +optional
	CABundle *string `json:"caBundle,omitempty"`
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesSettings `json:"kubernetes"`
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	// +patchMergeKey=name
	// +patchStrategy=merge
	MachineImages []MachineImage `json:"machineImages" patchStrategy:"merge" patchMergeKey:"name"`
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	// +patchMergeKey=name
	// +patchStrategy=merge
	MachineTypes []MachineType `json:"machineTypes" patchStrategy:"merge" patchMergeKey:"name"`
	// ProviderConfig contains provider-specific configuration for the profile.
	// +optional
	ProviderConfig *ProviderConfig `json:"providerConfig,omitempty"`
	// Regions contains constraints regarding allowed values for regions and zones.
	// +patchMergeKey=name
	// +patchStrategy=merge
	Regions []Region `json:"regions" patchStrategy:"merge" patchMergeKey:"name"`
	// SeedSelector contains an optional list of labels on `Seed` resources that marks those seeds whose shoots may use this provider profile.
	// An empty list means that all seeds of the same provider type are supported.
	// This is useful for environments that are of the same type (like openstack) but may have different "instances"/landscapes.
	// +optional
	SeedSelector *metav1.LabelSelector `json:"seedSelector,omitempty"`
	// Type is the name of the provider.
	Type string `json:"type"`
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	VolumeTypes []VolumeType `json:"volumeTypes,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

// KubernetesSettings contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
type KubernetesSettings struct {
	// Versions is the list of allowed Kubernetes versions with optional expiration dates for Shoot clusters.
	// +patchMergeKey=version
	// +patchStrategy=merge
	// +optional
	Versions []ExpirableVersion `json:"versions,omitempty" patchStrategy:"merge" patchMergeKey:"version"`
}

// MachineImage defines the name and multiple versions of the machine image in any environment.
type MachineImage struct {
	// Name is the name of the image.
	Name string `json:"name"`
	// Versions contains versions and expiration dates of the machine image
	// +patchMergeKey=version
	// +patchStrategy=merge
	Versions []ExpirableVersion `json:"versions" patchStrategy:"merge" patchMergeKey:"version"`
}

// ExpirableVersion contains a version and an expiration date.
type ExpirableVersion struct {
	// Version is the version identifier.
	Version string `json:"version"`
	// ExpirationDate defines the time at which this version expires.
	// +optional
	ExpirationDate *metav1.Time `json:"expirationDate,omitempty"`
}

// MachineType contains certain properties of a machine type.
type MachineType struct {
	// CPU is the number of CPUs for this machine type.
	CPU resource.Quantity `json:"cpu"`
	// GPU is the number of GPUs for this machine type.
	GPU resource.Quantity `json:"gpu"`
	// Memory is the amount of memory for this machine type.
	Memory resource.Quantity `json:"memory"`
	// Name is the name of the machine type.
	Name string `json:"name"`
	// Storage is the amount of storage associated with the root volume of this machine type.
	// +optional
	Storage *MachineTypeStorage `json:"storage,omitempty"`
	// Usable defines if the machine type can be used for shoot clusters.
	// +optional
	Usable *bool `json:"usable,omitempty"`
}

// MachineTypeStorage is the amount of storage associated with the root volume of this machine type.
type MachineTypeStorage struct {
	// Class is the class of the storage type.
	Class string `json:"class"`
	// Size is the storage size.
	Size resource.Quantity `json:"size"`
	// Type is the type of the storage.
	Type string `json:"type"`
}

// Region contains certain properties of a region.
type Region struct {
	// Name is a region name.
	Name string `json:"name"`
	// Zones is a list of availability zones in this region.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	Zones []AvailabilityZone `json:"zones,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

// AvailabilityZone is an availability zone.
type AvailabilityZone struct {
	// Name is an an availability zone name.
	Name string `json:"name"`
	// UnavailableMachineTypes is a list of machine type names that are not availability in this zone.
	// +optional
	UnavailableMachineTypes []string `json:"unavailableMachineTypes,omitempty"`
	// UnavailableVolumeTypes is a list of volume type names that are not availability in this zone.
	// +optional
	UnavailableVolumeTypes []string `json:"unavailableVolumeTypes,omitempty"`
}

// VolumeType contains certain properties of a volume type.
type VolumeType struct {
	// Class is the class of the volume type.
	Class string `json:"class"`
	// Name is the name of the volume type.
	Name string `json:"name"`
	// Usable defines if the volume type can be used for shoot clusters.
	// +optional
	Usable *bool `json:"usable,omitempty"`
}

const (
	// VolumeClassStandard is a constant for the standard volume class.
	VolumeClassStandard string = "standard"
	// VolumeClassPremium is a constant for the premium volume class.
	VolumeClassPremium string = "premium"
)
