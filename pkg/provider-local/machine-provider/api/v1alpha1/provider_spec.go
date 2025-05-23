// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

const (
	// V1Alpha1 is the API version.
	V1Alpha1 = "mcm.gardener.cloud/v1alpha1"
	// Provider is a constant for the provider name.
	Provider = "local"
)

// ProviderSpec is the spec to be used while parsing the calls.
type ProviderSpec struct {
	// APIVersion determines the API version for the provider APIs.
	APIVersion string `json:"apiVersion,omitempty"`
	// Image is the container image to use for the node.
	Image string `json:"image,omitempty"`
	// IPPoolNameV4 is the name of the crd.projectcalico.org/v1.IPPool that should be used for machine pods for IPv4
	// addresses.
	IPPoolNameV4 string `json:"ipPoolNameV4,omitempty"`
	// IPPoolNameV6 is the name of the crd.projectcalico.org/v1.IPPool that should be used for machine pods for IPv6
	// addresses.
	IPPoolNameV6 string `json:"ipPoolNameV6,omitempty"`
}
