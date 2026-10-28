// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// GardenerSeedLeaseNamespace is the namespace in which Gardenlet will report Seeds'
	// status using Lease resources for each Seed
	GardenerSeedLeaseNamespace = "gardener-system-seed-lease"
	// GardenerShootIssuerNamespace is the namespace in which Gardenlet
	// will sync service account issuer discovery documents
	// of Shoot clusters which require managed issuer
	GardenerShootIssuerNamespace = "gardener-system-shoot-issuer"
	// GardenerSystemPublicNamespace is the namespace which will contain a resources
	// describing gardener installation itself. The resources in this namespace
	// may be visible to all authenticated users.
	GardenerSystemPublicNamespace = "gardener-system-public"
)

// IPFamily is a type for specifying an IP protocol version to use in Gardener clusters.
type IPFamily string

const (
	// IPFamilyIPv4 is the IPv4 IP family.
	IPFamilyIPv4 IPFamily = "IPv4"
	// IPFamilyIPv6 is the IPv6 IP family.
	IPFamilyIPv6 IPFamily = "IPv6"
)

// IsIPv4SingleStack determines whether the given list of IP families specifies IPv4 single-stack networking.
func IsIPv4SingleStack(ipFamilies []IPFamily) bool {
	return len(ipFamilies) == 0 || (len(ipFamilies) == 1 && ipFamilies[0] == IPFamilyIPv4)
}

// IsIPv6SingleStack determines whether the given list of IP families specifies IPv6 single-stack networking.
func IsIPv6SingleStack(ipFamilies []IPFamily) bool {
	return len(ipFamilies) == 1 && ipFamilies[0] == IPFamilyIPv6
}

// IsDualStack determines whether the given list of IP families specifies dual-stack networking (both IPv4 and IPv6).
func IsDualStack(ipFamilies []IPFamily) bool {
	hasIPv4 := false
	hasIPv6 := false
	for _, family := range ipFamilies {
		switch family {
		case IPFamilyIPv4:
			hasIPv4 = true
		case IPFamilyIPv6:
			hasIPv6 = true
		}
	}
	return hasIPv4 && hasIPv6
}

// AccessRestriction describes an access restriction for a Kubernetes cluster (e.g., EU access-only).
type AccessRestriction struct {
	// Name is the name of the restriction.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
}

// AccessRestrictionWithOptions describes an access restriction for a Kubernetes cluster (e.g., EU access-only) and
// allows to specify additional options.
type AccessRestrictionWithOptions struct {
	AccessRestriction `json:",inline" protobuf:"bytes,1,opt,name=accessRestriction"`

	// Options is a map of additional options for the access restriction.
	// +optional
	Options map[string]string `json:"options,omitempty" protobuf:"bytes,2,rep,name=options"`
}

// Extension contains type and provider information for extensions.
type Extension struct {
	// Type is the type of the extension resource.
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// ProviderConfig is the configuration passed to extension resource.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,2,opt,name=providerConfig"`
	// Disabled allows to disable extensions that were marked as 'automatically enabled' by Gardener administrators.
	// +optional
	Disabled *bool `json:"disabled,omitempty" protobuf:"varint,3,opt,name=disabled"`
	// AutoUpdate is the default auto-update policy for the components of this extension. It applies to
	// every component that does not define its own autoUpdate. Only relevant for extension types that
	// publish an ExtensionProfile.
	// +optional
	AutoUpdate *ExtensionAutoUpdate `json:"autoUpdate,omitempty" protobuf:"bytes,4,opt,name=autoUpdate"`
	// Components pins the versions of the components managed by this extension and optionally overrides
	// the auto-update policy per component. A pinned version is validated against the matching
	// ExtensionProfile. Only relevant for extension types that publish an ExtensionProfile.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Components []ShootExtensionComponent `json:"components,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,5,rep,name=components"`
}

// ExtensionAutoUpdate defines the auto-update policy for the components of an extension.
type ExtensionAutoUpdate struct {
	// Enabled indicates whether components are automatically upgraded within the configured update strategy.
	// When disabled, a component only moves off its pinned version once that version expires (force-upgrade).
	Enabled bool `json:"enabled" protobuf:"varint,1,opt,name=enabled"`
	// UpdateStrategy is the auto-update strategy to use while enabled. Possible values are: patch, minor, major.
	// +optional
	UpdateStrategy *MachineImageUpdateStrategy `json:"updateStrategy,omitempty" protobuf:"bytes,2,opt,name=updateStrategy,casttype=MachineImageUpdateStrategy"`
}

// ShootExtensionComponent pins the version of a single component managed by an extension.
type ShootExtensionComponent struct {
	// Name is the name of the component as listed in the ExtensionProfile.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Version pins the component version. A partial version is resolved to the highest matching supported
	// version at admission. When omitted, the extension behaves as before this field existed.
	// +optional
	Version *string `json:"version,omitempty" protobuf:"bytes,2,opt,name=version"`
	// AutoUpdate overrides the extension-level auto-update policy for this component. Optional.
	// +optional
	AutoUpdate *ExtensionAutoUpdate `json:"autoUpdate,omitempty" protobuf:"bytes,3,opt,name=autoUpdate"`
	// UpdateStrategy overrides the resolved update strategy for this component. Possible values are:
	// patch, minor, major.
	// +optional
	UpdateStrategy *MachineImageUpdateStrategy `json:"updateStrategy,omitempty" protobuf:"bytes,4,opt,name=updateStrategy,casttype=MachineImageUpdateStrategy"`
}

// NamedResourceReference is a named reference to a resource.
type NamedResourceReference struct {
	// Name of the resource reference.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// ResourceRef is a reference to a resource.
	ResourceRef autoscalingv1.CrossVersionObjectReference `json:"resourceRef" protobuf:"bytes,2,opt,name=resourceRef"`
}

// EncryptionProviderType is a type alias for the encryption provider type string.
type EncryptionProviderType string

const (
	// EncryptionProviderTypeAESCBC is the aescbc encryption provider type.
	EncryptionProviderTypeAESCBC EncryptionProviderType = "aescbc"
	// EncryptionProviderTypeAESGCM is the aesgcm encryption provider type.
	EncryptionProviderTypeAESGCM EncryptionProviderType = "aesgcm"
	// EncryptionProviderTypeSecretbox is the secretbox encryption provider type.
	EncryptionProviderTypeSecretbox EncryptionProviderType = "secretbox"
)
