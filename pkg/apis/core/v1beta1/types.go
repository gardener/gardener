// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"encoding/json"
	"strings"
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

// CapabilityValues is a list of values for a capability.
// The type is wrapped to represent the values as a comma-separated string in JSON.
type CapabilityValues struct {
	Values []string `protobuf:"bytes,1,rep,name=values"`
}

// Capabilities of a machine type or machine image.
type Capabilities map[string]CapabilityValues

// CapabilitiesSetCapabilities is a wrapper for Capabilities
// this is a workaround as we cannot define a slice of maps in protobuf
// we define custom marshal/unmarshal functions to get around this l
// If there is a way to avoid this, we should do it.
type CapabilitiesSetCapabilities struct {
	Capabilities Capabilities `json:"-"`
}

// MarshalJSON marshals the CapabilitiesSetCapabilities object to JSON.
func (c *CapabilitiesSetCapabilities) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Capabilities)
}

// UnmarshalJSON unmarshals the CapabilitiesSetCapabilities object from JSON.
func (c *CapabilitiesSetCapabilities) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &c.Capabilities)
}

// UnmarshalJSON unmarshals the CapabilityValues object from JSON.
func (c *CapabilityValues) UnmarshalJSON(bytes []byte) error {
	var str string
	if err := json.Unmarshal(bytes, &str); err != nil {
		return err
	}
	rawValues := strings.Split(str, ",")

	for _, value := range rawValues {
		c.Values = append(c.Values, strings.TrimSpace(value))
	}

	return nil
}

// MarshalJSON marshals the CapabilityValues object to JSON.
func (c CapabilityValues) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strings.Join(c.Values, ",") + `"`), nil
}

// HasEntries checks if a Capability is defined.
func (capabilities Capabilities) HasEntries() bool {
	return len(capabilities) != 0
}
