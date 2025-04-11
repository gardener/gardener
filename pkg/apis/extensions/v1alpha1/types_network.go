// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ Object = (*Network)(nil)

// NetworkResource is a constant for the name of the Network resource.
const NetworkResource = "Network"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,path=networks,singular=network
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Type,JSONPath=".spec.type",type=string,description="The type of the network provider for this resource."
// +kubebuilder:printcolumn:name=Pod CIDR,JSONPath=".spec.podCIDR",type=string,description="The CIDR that will be used for pods."
// +kubebuilder:printcolumn:name=Service CIDR,JSONPath=".spec.serviceCIDR",type=string,description="The CIDR that will be used for services."
// +kubebuilder:printcolumn:name=Status,JSONPath=".status.lastOperation.state",type=string,description="Status of network resource."
// +kubebuilder:printcolumn:name=Age,JSONPath=".metadata.creationTimestamp",type=date,description="creation timestamp"

// Network is the specification for cluster networking.
type Network struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the Network.
	// If the object's deletion timestamp is set, this field is immutable.
	Spec NetworkSpec `json:"spec"`
	// +optional
	Status NetworkStatus `json:"status"`
}

// GetExtensionSpec implements Object.
func (n *Network) GetExtensionSpec() Spec {
	return &n.Spec
}

// GetExtensionStatus implements Object.
func (n *Network) GetExtensionStatus() Status {
	return &n.Status
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkList is a list of Network resources.
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of Networks.
	Items []Network `json:"items"`
}

// NetworkSpec is the spec for an Network resource.
type NetworkSpec struct {
	// DefaultSpec is a structure containing common fields used by all extension resources.
	DefaultSpec `json:",inline"`
	// PodCIDR defines the CIDR that will be used for pods. This field is immutable.
	PodCIDR string `json:"podCIDR"`
	// ServiceCIDR defines the CIDR that will be used for services. This field is immutable.
	ServiceCIDR string `json:"serviceCIDR"`
	// IPFamilies specifies the IP protocol versions to use for shoot networking.
	// See https://github.com/gardener/gardener/blob/master/docs/development/ipv6.md
	// +optional
	IPFamilies []IPFamily `json:"ipFamilies,omitempty"`
}

// NetworkStatus is the status for an Network resource.
type NetworkStatus struct {
	// DefaultStatus is a structure containing common fields used by all extension resources.
	DefaultStatus `json:",inline"`
	// IPFamilies specifies the IP protocol versions that actually are used for shoot networking.
	// During dual-stack migration, this field may differ from the spec.
	// +optional
	IPFamilies []IPFamily `json:"ipFamilies,omitempty"`
}

// GetExtensionType returns the type of this Network resource.
func (n *Network) GetExtensionType() string {
	return n.Spec.Type
}
