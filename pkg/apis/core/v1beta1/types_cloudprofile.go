// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"encoding/json"
	"fmt"
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
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Spec defines the provider environment properties.
	// +optional
	Spec CloudProfileSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfileList is a collection of CloudProfiles.
type CloudProfileList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of CloudProfiles.
	Items []CloudProfile `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// CloudProfileSpec is the specification of a CloudProfile.
// It must contain exactly one of its defined keys.
type CloudProfileSpec struct {
	// CABundle is a certificate bundle which will be installed onto every host machine of shoot cluster targeting this profile.
	// +optional
	CABundle *string `json:"caBundle,omitempty" protobuf:"bytes,1,opt,name=caBundle"`
	// Kubernetes contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
	Kubernetes KubernetesSettings `json:"kubernetes" protobuf:"bytes,2,opt,name=kubernetes"`
	// MachineImages contains constraints regarding allowed values for machine images in the Shoot specification.
	// +patchMergeKey=name
	// +patchStrategy=merge
	MachineImages []MachineImage `json:"machineImages" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,3,rep,name=machineImages"`
	// MachineTypes contains constraints regarding allowed values for machine types in the 'workers' block in the Shoot specification.
	// +patchMergeKey=name
	// +patchStrategy=merge
	MachineTypes []MachineType `json:"machineTypes" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,4,rep,name=machineTypes"`
	// ProviderConfig contains provider-specific configuration for the profile.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,5,opt,name=providerConfig"`
	// Regions contains constraints regarding allowed values for regions and zones.
	// +patchMergeKey=name
	// +patchStrategy=merge
	Regions []Region `json:"regions" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,6,rep,name=regions"`
	// SeedSelector contains an optional list of labels on `Seed` resources that marks those seeds whose shoots may use this provider profile.
	// An empty list means that all seeds of the same provider type are supported.
	// This is useful for environments that are of the same type (like openstack) but may have different "instances"/landscapes.
	// Optionally a list of possible providers can be added to enable cross-provider scheduling. By default, the provider
	// type of the seed must match the shoot's provider.
	// +optional
	SeedSelector *SeedSelector `json:"seedSelector,omitempty" protobuf:"bytes,7,opt,name=seedSelector"`
	// Type is the name of the provider.
	Type string `json:"type" protobuf:"bytes,8,opt,name=type"`
	// VolumeTypes contains constraints regarding allowed values for volume types in the 'workers' block in the Shoot specification.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	VolumeTypes []VolumeType `json:"volumeTypes,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,9,rep,name=volumeTypes"`
	// Bastion contains the machine and image properties
	// +optional
	Bastion *Bastion `json:"bastion,omitempty" protobuf:"bytes,10,opt,name=bastion"`
	// Limits configures operational limits for Shoot clusters using this CloudProfile.
	// See https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md.
	// +optional
	Limits *Limits `json:"limits,omitempty" protobuf:"bytes,11,opt,name=limits"`
	// Capabilities contains the definition of all possible capabilities in the CloudProfile.
	// Only capabilities and values defined here can be used to describe MachineImages and MachineTypes.
	// The order of values for a given capability is relevant. The most important value is listed first.
	// During maintenance upgrades, the image that matches most capabilities will be selected.
	// +optional
	Capabilities []Capability `json:"capabilities,omitempty" protobuf:"bytes,12,rep,name=capabilities"`
}

// GetCapabilities returns the capabilities slice of the CloudProfile as a Capabilities map.
func (spec *CloudProfileSpec) GetCapabilities() Capabilities {
	if len(spec.Capabilities) == 0 {
		return nil
	}
	capabilities := make(Capabilities, len(spec.Capabilities))
	for _, capability := range spec.Capabilities {
		capabilities[capability.Name] = capability.Values
	}
	return capabilities
}

// SeedSelector contains constraints for selecting seed to be usable for shoots using a profile
type SeedSelector struct {
	// LabelSelector is optional and can be used to select seeds by their label settings
	// +optional
	metav1.LabelSelector `json:",inline,omitempty" protobuf:"bytes,1,opt,name=labelSelector"`
	// Providers is optional and can be used by restricting seeds by their provider type. '*' can be used to enable seeds regardless of their provider type.
	// +optional
	ProviderTypes []string `json:"providerTypes,omitempty" protobuf:"bytes,2,rep,name=providerTypes"`
}

// KubernetesSettings contains constraints regarding allowed values of the 'kubernetes' block in the Shoot specification.
type KubernetesSettings struct {
	// Versions is the list of allowed Kubernetes versions with optional expiration dates for Shoot clusters.
	// +patchMergeKey=version
	// +patchStrategy=merge
	// +optional
	Versions []ExpirableVersion `json:"versions,omitempty" patchStrategy:"merge" patchMergeKey:"version" protobuf:"bytes,1,rep,name=versions"`
}

// MachineImage defines the name and multiple versions of the machine image in any environment.
type MachineImage struct {
	// Name is the name of the image.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Versions contains versions, expiration dates and container runtimes of the machine image
	// +patchMergeKey=version
	// +patchStrategy=merge
	Versions []MachineImageVersion `json:"versions" patchStrategy:"merge" patchMergeKey:"version" protobuf:"bytes,2,rep,name=versions"`
	// UpdateStrategy is the update strategy to use for the machine image. Possible values are:
	//  - patch: update to the latest patch version of the current minor version.
	//  - minor: update to the latest minor and patch version.
	//  - major: always update to the overall latest version (default).
	// +optional
	UpdateStrategy *MachineImageUpdateStrategy `json:"updateStrategy,omitempty" protobuf:"bytes,3,opt,name=updateStrategy,casttype=MachineImageUpdateStrategy"`
}

// MachineImageVersion is an expirable version with list of supported container runtimes and interfaces
type MachineImageVersion struct {
	ExpirableVersion `json:",inline" protobuf:"bytes,1,opt,name=expirableVersion"`
	// CRI list of supported container runtime and interfaces supported by this version
	// +optional
	CRI []CRI `json:"cri,omitempty" protobuf:"bytes,2,rep,name=cri"`
	// Architectures is the list of CPU architectures of the machine image in this version.
	// +optional
	Architectures []string `json:"architectures,omitempty" protobuf:"bytes,3,opt,name=architectures"`
	// KubeletVersionConstraint is a constraint describing the supported kubelet versions by the machine image in this version.
	// If the field is not specified, it is assumed that the machine image in this version supports all kubelet versions.
	// Examples:
	// - '>= 1.26' - supports only kubelet versions greater than or equal to 1.26
	// - '< 1.26' - supports only kubelet versions less than 1.26
	// +optional
	KubeletVersionConstraint *string `json:"kubeletVersionConstraint,omitempty" protobuf:"bytes,4,opt,name=kubeletVersionConstraint"`
	// InPlaceUpdates contains the configuration for in-place updates for this machine image version.
	// +optional
	InPlaceUpdates *InPlaceUpdates `json:"inPlaceUpdates,omitempty" protobuf:"bytes,5,opt,name=inPlaceUpdates"`
	// CapabilitySets is an array of capability sets. Each entry represents a combination of capabilities that is provided by
	// the machine image version.
	// +optional
	CapabilitySets []CapabilitySet `json:"capabilitySets,omitempty" protobuf:"bytes,6,rep,name=capabilitySets"`
}

// SupportsArchitecture checks if the machine image version supports the given architecture.
func (v *MachineImageVersion) SupportsArchitecture(capabilities Capabilities, architecture string) bool {
	if len(capabilities) == 0 {
		return slices.Contains(v.Architectures, architecture)
	}
	for _, capability := range v.CapabilitySets {
		if slices.Contains(capability.Capabilities[constants.ArchitectureKey], architecture) {
			return true
		}
	}
	return slices.Contains(capabilities[constants.ArchitectureKey], architecture)
}

// GetArchitectures returns the list of supported architectures for the machine image version.
func (v *MachineImageVersion) GetArchitectures(capabilities Capabilities) []string {
	if len(capabilities) > 0 {
		return ExtractArchitectures(v.CapabilitySets)
	}
	return v.Architectures
}

// ExpirableVersion contains a version and an expiration date.
type ExpirableVersion struct {
	// Version is the version identifier.
	Version string `json:"version" protobuf:"bytes,1,opt,name=version"`
	// ExpirationDate defines the time at which this version expires.
	// +optional
	ExpirationDate *metav1.Time `json:"expirationDate,omitempty" protobuf:"bytes,2,opt,name=expirationDate"`
	// Classification defines the state of a version (preview, supported, deprecated)
	// +optional
	Classification *VersionClassification `json:"classification,omitempty" protobuf:"bytes,3,opt,name=classification,casttype=VersionClassification"`
}

// MachineType contains certain properties of a machine type.
type MachineType struct {
	// CPU is the number of CPUs for this machine type.
	CPU resource.Quantity `json:"cpu" protobuf:"bytes,1,opt,name=cpu"`
	// GPU is the number of GPUs for this machine type.
	GPU resource.Quantity `json:"gpu" protobuf:"bytes,2,opt,name=gpu"`
	// Memory is the amount of memory for this machine type.
	Memory resource.Quantity `json:"memory" protobuf:"bytes,3,opt,name=memory"`
	// Name is the name of the machine type.
	Name string `json:"name" protobuf:"bytes,4,opt,name=name"`
	// Storage is the amount of storage associated with the root volume of this machine type.
	// +optional
	Storage *MachineTypeStorage `json:"storage,omitempty" protobuf:"bytes,5,opt,name=storage"`
	// Usable defines if the machine type can be used for shoot clusters.
	// +optional
	Usable *bool `json:"usable,omitempty" protobuf:"varint,6,opt,name=usable"`
	// Architecture is the CPU architecture of this machine type.
	// +optional
	Architecture *string `json:"architecture,omitempty" protobuf:"bytes,7,opt,name=architecture"`
	// Capabilities contains the the machine type capabilities.
	// +optional
	Capabilities Capabilities `json:"capabilities,omitempty" protobuf:"bytes,8,rep,name=capabilities,casttype=Capabilities"`
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
	Class string `json:"class" protobuf:"bytes,1,opt,name=class"`
	// StorageSize is the storage size.
	// +optional
	StorageSize *resource.Quantity `json:"size,omitempty" protobuf:"bytes,2,opt,name=size"`
	// Type is the type of the storage.
	Type string `json:"type" protobuf:"bytes,3,opt,name=type"`
	// MinSize is the minimal supported storage size.
	// This overrides any other common minimum size configuration from `spec.volumeTypes[*].minSize`.
	// +optional
	MinSize *resource.Quantity `json:"minSize,omitempty" protobuf:"bytes,4,opt,name=minSize"`
}

// Region contains certain properties of a region.
type Region struct {
	// Name is a region name.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Zones is a list of availability zones in this region.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	Zones []AvailabilityZone `json:"zones,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,2,rep,name=zones"`
	// Labels is an optional set of key-value pairs that contain certain administrator-controlled labels for this region.
	// It can be used by Gardener administrators/operators to provide additional information about a region, e.g. wrt
	// quality, reliability, etc.
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,3,rep,name=labels"`
	// AccessRestrictions describe a list of access restrictions that can be used for Shoots using this region.
	// +optional
	AccessRestrictions []AccessRestriction `json:"accessRestrictions,omitempty" protobuf:"bytes,4,rep,name=accessRestrictions"`
}

// AvailabilityZone is an availability zone.
type AvailabilityZone struct {
	// Name is an availability zone name.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// UnavailableMachineTypes is a list of machine type names that are not availability in this zone.
	// +optional
	UnavailableMachineTypes []string `json:"unavailableMachineTypes,omitempty" protobuf:"bytes,2,rep,name=unavailableMachineTypes"`
	// UnavailableVolumeTypes is a list of volume type names that are not availability in this zone.
	// +optional
	UnavailableVolumeTypes []string `json:"unavailableVolumeTypes,omitempty" protobuf:"bytes,3,rep,name=unavailableVolumeTypes"`
}

// VolumeType contains certain properties of a volume type.
type VolumeType struct {
	// Class is the class of the volume type.
	Class string `json:"class" protobuf:"bytes,1,opt,name=class"`
	// Name is the name of the volume type.
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`
	// Usable defines if the volume type can be used for shoot clusters.
	// +optional
	Usable *bool `json:"usable,omitempty" protobuf:"varint,3,opt,name=usable"`
	// MinSize is the minimal supported storage size.
	// +optional
	MinSize *resource.Quantity `json:"minSize,omitempty" protobuf:"bytes,4,opt,name=minSize"`
}

// Bastion contains the bastions creation info
type Bastion struct {
	// MachineImage contains the bastions machine image properties
	// +optional
	MachineImage *BastionMachineImage `json:"machineImage,omitempty" protobuf:"bytes,1,opt,name=machineImage"`
	// MachineType contains the bastions machine type properties
	// +optional
	MachineType *BastionMachineType `json:"machineType,omitempty" protobuf:"bytes,2,opt,name=machineType"`
}

// BastionMachineImage contains the bastions machine image properties
type BastionMachineImage struct {
	// Name of the machine image
	Name string `json:"name" protobuf:"bytes,1,name=name"`
	// Version of the machine image
	// +optional
	Version *string `json:"version,omitempty" protobuf:"bytes,2,opt,name=version"`
}

// BastionMachineType contains the bastions machine type properties
type BastionMachineType struct {
	// Name of the machine type
	Name string `json:"name" protobuf:"bytes,1,name=name"`
}

// Limits configures operational limits for Shoot clusters using this CloudProfile.
// See https://github.com/gardener/gardener/blob/master/docs/usage/shoot/shoot_limits.md.
type Limits struct {
	// MaxNodesTotal configures the maximum node count a Shoot cluster can have during runtime.
	// +optional
	MaxNodesTotal *int32 `json:"maxNodesTotal,omitempty" protobuf:"varint,1,opt,name=maxNodesTotal"`
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
	Supported bool `json:"supported" protobuf:"varint,1,opt,name=supported"`
	// MinVersionForInPlaceUpdate specifies the minimum supported version from which an in-place update to this machine image version can be performed.
	// +optional
	MinVersionForUpdate *string `json:"minVersionForUpdate,omitempty" protobuf:"bytes,2,opt,name=minVersionForUpdate"`
}

// CapabilityValues contains capability values.
// This is a workaround as the Protobuf generator can't handle a map with slice values.
// +protobuf.nullable=true
// +protobuf.options.(gogoproto.goproto_stringer)=false
type CapabilityValues []string

func (t CapabilityValues) String() string {
	return fmt.Sprintf("%v", []string(t))
}

// Capabilities of a machine type or machine image.
// +protobuf.options.(gogoproto.goproto_stringer)=false
type Capabilities map[string]CapabilityValues

func (t Capabilities) String() string {
	return fmt.Sprintf("%v", map[string]CapabilityValues(t))
}

// Capability contains the Name and Values of a capability.
type Capability struct {
	Name   string   `json:"name" protobuf:"bytes,1,opt,name=name"`
	Values []string `json:"values" protobuf:"bytes,2,rep,name=values"`
}

// CapabilitySet is a wrapper for Capabilities.
// This is a workaround as the Protobuf generator can't handle a slice of maps.
type CapabilitySet struct {
	Capabilities `json:"-" protobuf:"bytes,1,rep,name=capabilities,casttype=Capabilities"`
}

// UnmarshalJSON unmarshals the given data to a CapabilitySet.
func (c *CapabilitySet) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &c.Capabilities)
}

// MarshalJSON marshals the CapabilitySet object to JSON.
func (c *CapabilitySet) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Capabilities)
}

// ExtractArchitectures extracts the architectures from the given capability sets.
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
