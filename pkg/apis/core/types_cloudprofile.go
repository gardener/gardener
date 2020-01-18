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

package core

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfile represents certain properties about a provider environment.
type CloudProfile struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec defines the provider environment properties.
	Spec CloudProfileSpec
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfileList is a collection of CloudProfiles.
type CloudProfileList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of CloudProfiles.
	Items []CloudProfile
}

// CloudProfileSpec is the specification of a CloudProfile.
// It must contain exactly one of its defined keys.
type CloudProfileSpec struct {
	// CABundle is a certificate bundle which will be installed onto every host machine of shoot cluster targeting this profile.
	CABundle *string
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesSettings
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	MachineImages []MachineImage
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	MachineTypes []MachineType
	// ProviderConfig contains provider-specific configuration for the profile.
	ProviderConfig *ProviderConfig
	// Regions contains constraints regarding allowed values for regions and zones.
	Regions []Region
	// SeedSelector contains an optional list of labels on `Seed` resources that marks those seeds whose shoots may use this provider profile.
	// An empty list means that all seeds of the same provider type are supported.
	// This is useful for environments that are of the same type (like openstack) but may have different "instances"/landscapes.
	SeedSelector *metav1.LabelSelector
	// Type is the name of the provider.
	Type string
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType
}

// KubernetesSettings contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
type KubernetesSettings struct {
	// Versions is the list of allowed Kubernetes versions with optional expiration dates for Shoot clusters.
	Versions []ExpirableVersion
}

// MachineImage defines the name and multiple versions of the machine image in any environment.
type MachineImage struct {
	// Name is the name of the image.
	Name string
	// Versions contains versions and expiration dates of the machine image
	Versions []ExpirableVersion
}

// ExpirableVersion contains a version and an expiration date.
type ExpirableVersion struct {
	// Version is the version identifier.
	Version string
	// ExpirationDate defines the time at which this version expires.
	ExpirationDate *metav1.Time
}

// MachineType contains certain properties of a machine type.
type MachineType struct {
	// CPU is the number of CPUs for this machine type.
	CPU resource.Quantity
	// GPU is the number of GPUs for this machine type.
	GPU resource.Quantity
	// Memory is the amount of memory for this machine type.
	Memory resource.Quantity
	// Name is the name of the machine type.
	Name string
	// Storage is the amount of storage associated with the root volume of this machine type.
	Storage *MachineTypeStorage
	// Usable defines if the machine type can be used for shoot clusters.
	Usable *bool
}

// MachineTypeStorage is the amount of storage associated with the root volume of this machine type.
type MachineTypeStorage struct {
	// Class is the class of the storage type.
	Class string
	// StorageSize is the storage size.
	StorageSize resource.Quantity
	// Type is the type of the storage.
	Type string
}

// Region contains certain properties of a region.
type Region struct {
	// Name is a region name.
	Name string
	// Zones is a list of availability zones in this region.
	Zones []AvailabilityZone
}

// AvailabilityZone is an availability zone.
type AvailabilityZone struct {
	// Name is an an availability zone name.
	Name string
	// UnavailableMachineTypes is a list of machine type names that are not availability in this zone.
	UnavailableMachineTypes []string
	// UnavailableVolumeTypes is a list of volume type names that are not availability in this zone.
	UnavailableVolumeTypes []string
}

// VolumeType contains certain properties of a volume type.
type VolumeType struct {
	// Class is the class of the volume type.
	Class string
	// Name is the name of the volume type.
	Name string
	// Usable defines if the volume type can be used for shoot clusters.
	Usable *bool
}

const (
	// VolumeClassStandard is a constant for the standard volume class.
	VolumeClassStandard string = "standard"
	// VolumeClassPremium is a constant for the premium volume class.
	VolumeClassPremium string = "premium"
)
