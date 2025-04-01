// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"slices"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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
	ProviderConfig *runtime.RawExtension
	// Regions contains constraints regarding allowed values for regions and zones.
	Regions []Region
	// SeedSelector contains an optional list of labels on `Seed` resources that marks those seeds whose shoots may use this provider profile.
	// An empty list means that all seeds of the same provider type are supported.
	// This is useful for environments that are of the same type (like openstack) but may have different "instances"/landscapes.
	// Optionally a list of possible providers can be added to enable cross-provider scheduling. By default, the provider
	// type of the seed must match the shoot's provider.
	SeedSelector *SeedSelector
	// Type is the name of the provider.
	Type string
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	VolumeTypes []VolumeType
	// Bastion contains machine and image properties
	Bastion *Bastion
	// Limits configures operational limits for Shoot clusters using this CloudProfile.
	// See https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md.
	Limits *Limits
	// Capabilities contains the definition of all possible capabilities in the CloudProfile.
	// Only capabilities and values defined here can be used to describe MachineImages and MachineTypes.
	// The order of values for a given capability is relevant. The most important value is listed first.
	// During maintenance upgrades, the image that matches most capabilities will be selected.
	Capabilities map[string]CapabilityValues
}

// SeedSelector contains constraints for selecting seed to be usable for shoots using a profile
type SeedSelector struct {
	// LabelSelector is optional and can be used to select seeds by their label settings
	metav1.LabelSelector
	// ProviderTypes contains a list of allowed provider types used by the Gardener scheduler to restricting seeds by
	// their provider type and enable cross-provider scheduling.
	// By default, Shoots are only scheduled on Seeds having the same provider type.
	ProviderTypes []string
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
	// Versions contains versions, expiration dates and container runtimes of the machine image
	Versions []MachineImageVersion
	// UpdateStrategy is the update strategy to use for the machine image. Possible values are:
	//  - patch: update to the latest patch version of the current minor version.
	//  - minor: update to the latest minor and patch version.
	//  - major: always update to the overall latest version (default).
	UpdateStrategy *MachineImageUpdateStrategy
}

// MachineImageVersion is an expirable version with list of supported container runtimes and interfaces
type MachineImageVersion struct {
	ExpirableVersion
	// CRI list of supported container runtime and interfaces supported by this version
	CRI []CRI
	// Architectures is the list of CPU architectures of the machine image in this version.
	Architectures []string
	// KubeletVersionConstraint is a constraint describing the supported kubelet versions by the machine image in this version.
	// If the field is not specified, it is assumed that the machine image in this version supports all kubelet versions.
	// Examples:
	// - '>= 1.26' - supports only kubelet versions greater than or equal to 1.26
	// - '< 1.26' - supports only kubelet versions less than 1.26
	KubeletVersionConstraint *string
	// InPlaceUpdates contains the configuration for in-place updates for this machine image version.
	InPlaceUpdates *InPlaceUpdates
	// CapabilitySets is an array of capability sets. Each entry represents a combination of capabilities that is provided by
	// the machine image version.
	CapabilitySets []CapabilitySet
}

// SupportsArchitecture checks if the machine image version supports a given architecture.
func (m *MachineImageVersion) SupportsArchitecture(capabilities Capabilities, architecture string) bool {
	if len(capabilities) == 0 {
		return slices.Contains(m.Architectures, architecture)
	}
	for _, capability := range m.CapabilitySets {
		if slices.Contains(capability.Capabilities[constants.ArchitectureKey], architecture) {
			return true
		}
	}
	return slices.Contains(capabilities[constants.ArchitectureKey], architecture)
}

// ExpirableVersion contains a version and an expiration date.
type ExpirableVersion struct {
	// Version is the version identifier.
	Version string
	// ExpirationDate defines the time at which this version expires.
	ExpirationDate *metav1.Time
	// Classification defines the state of a version (preview, supported, deprecated)
	Classification *VersionClassification
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
	// Architecture is the CPU architecture of this machine type.
	Architecture *string
	// Capabilities contains the the machine type capabilities.
	Capabilities Capabilities
}

// GetArchitecture returns the architecture of the machine type.
func (m *MachineType) GetArchitecture() string {
	if len(m.Capabilities[constants.ArchitectureKey]) == 1 {
		return m.Capabilities[constants.ArchitectureKey][0]
	}
	return ptr.Deref(m.Architecture, "")
}

// MachineTypeStorage is the amount of storage associated with the root volume of this machine type.
type MachineTypeStorage struct {
	// Class is the class of the storage type.
	Class string
	// StorageSize is the storage size.
	StorageSize *resource.Quantity
	// Type is the type of the storage.
	Type string
	// MinSize is the minimal supported storage size.
	// This overrides any other common minimum size configuration in the `spec.volumeTypes[*].minSize`.
	MinSize *resource.Quantity
}

// Region contains certain properties of a region.
type Region struct {
	// Name is a region name.
	Name string
	// Zones is a list of availability zones in this region.
	Zones []AvailabilityZone
	// Labels is an optional set of key-value pairs that contain certain administrator-controlled labels for this region.
	// It can be used by Gardener administrators/operators to provide additional information about a region, e.g. wrt
	// quality, reliability, etc.
	Labels map[string]string
	// AccessRestrictions describe a list of access restrictions that can be used for Shoots using this region.
	AccessRestrictions []AccessRestriction
}

// AvailabilityZone is an availability zone.
type AvailabilityZone struct {
	// Name is an availability zone name.
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
	// MinSize is the minimal supported storage size.
	MinSize *resource.Quantity
}

// Bastion contains the bastions creation info
type Bastion struct {
	// MachineImage contains the bastions machine image properties
	MachineImage *BastionMachineImage
	// MachineType contains the bastions machine type properties
	MachineType *BastionMachineType
}

// BastionMachineImage contains the bastions machine image properties
type BastionMachineImage struct {
	// Name of the machine image
	Name string
	// Version of the machine image
	Version *string
}

// BastionMachineType contains the bastions machine type properties
type BastionMachineType struct {
	// Name of the machine type
	Name string
}

// Limits configures operational limits for Shoot clusters using this CloudProfile.
// See https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md.
type Limits struct {
	// MaxNodesTotal configures the maximum node count a Shoot cluster can have during runtime.
	MaxNodesTotal *int32
}

const (
	// VolumeClassStandard is a constant for the standard volume class.
	VolumeClassStandard string = "standard"
	// VolumeClassPremium is a constant for the premium volume class.
	VolumeClassPremium string = "premium"
)

// VersionClassification is the logical state of a version.
type VersionClassification string

const (
	// ClassificationPreview indicates that a version has recently been added and not promoted to "Supported" yet.
	// ClassificationPreview versions will not be considered for automatic Kubernetes and Machine Image patch version updates.
	ClassificationPreview VersionClassification = "preview"
	// ClassificationSupported indicates that a patch version is the recommended version for a shoot.
	// Only one "supported" version is allowed per minor version.
	// Supported versions are eligible for the automated Kubernetes and Machine image patch version update for shoot clusters in Gardener.
	ClassificationSupported VersionClassification = "supported"
	// ClassificationDeprecated indicates that a patch version should not be used anymore, should be updated to a new version
	// and will eventually expire.
	ClassificationDeprecated VersionClassification = "deprecated"
)

// MachineImageUpdateStrategy is the update strategy to use for a machine image
type MachineImageUpdateStrategy string

const (
	// UpdateStrategyPatch indicates that auto-updates are performed to the latest patch version of the current minor version.
	// When using an expired version during the maintenance window, force updates to the latest patch of the next (not necessarily consecutive) minor when using an expired version.
	UpdateStrategyPatch MachineImageUpdateStrategy = "patch"
	// UpdateStrategyMinor indicates that auto-updates are performed to the latest patch and minor version of the current major version.
	// When using an expired version during the maintenance window, force updates to the latest minor and patch of the next (not necessarily consecutive) major version.
	UpdateStrategyMinor MachineImageUpdateStrategy = "minor"
	// UpdateStrategyMajor indicates that auto-updates are performed always to the overall latest version.
	UpdateStrategyMajor MachineImageUpdateStrategy = "major"
)

// InPlaceUpdates contains the configuration for in-place updates for a machine image version.
type InPlaceUpdates struct {
	// Supported indicates whether in-place updates are supported for this machine image version.
	Supported bool
	// MinVersionForInPlaceUpdate specifies the minimum supported version from which an in-place update to this machine image version can be performed.
	MinVersionForUpdate *string
}

// CapabilityValues contains capability values.
// This is a workaround as the Protobuf generator can't handle a map with slice values.
type CapabilityValues []string

// Contains checks if the CapabilityValues contains all values.
func (c CapabilityValues) Contains(values ...string) bool {
	for _, value := range values {
		if !slices.Contains(c, value) {
			return false
		}
	}
	return true
}

// IsSubsetOf checks if the CapabilityValues is a subset of another CapabilityValues.
func (c CapabilityValues) IsSubsetOf(other CapabilityValues) bool {
	for _, value := range c {
		if !other.Contains(value) {
			return false
		}
	}
	return true
}

// Capabilities of a machine type or machine image.
type Capabilities map[string]CapabilityValues

// CapabilitySet is a wrapper for Capabilities.
// This is a workaround as the Protobuf generator can't handle a slice of maps.
type CapabilitySet struct {
	Capabilities
}

// ExtractArchitectures extracts all architectures from a list of CapabilitySets.
func ExtractArchitectures(capabilities []CapabilitySet) []string {
	var architectures []string
	for _, capabilitySet := range capabilities {
		for _, architectureValue := range capabilitySet.Capabilities[constants.ArchitectureKey] {
			if !slices.Contains(architectures, architectureValue) {
				architectures = append(architectures, architectureValue)
			}
		}
	}
	return architectures
}
