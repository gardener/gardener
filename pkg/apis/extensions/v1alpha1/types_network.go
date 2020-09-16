// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
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

// Network is the specification for cluster networking.
type Network struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec"`
	Status NetworkStatus `json:"status"`
}

// GetExtensionSpec implements Object.
func (i *Network) GetExtensionSpec() Spec {
	return &i.Spec
}

// GetExtensionStatus implements Object.
func (i *Network) GetExtensionStatus() Status {
	return &i.Status
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
	// PodCIDR defines the CIDR that will be used for pods.
	PodCIDR string `json:"podCIDR"`
	// ServiceCIDR defines the CIDR that will be used for services.
	ServiceCIDR string `json:"serviceCIDR"`
}

// NetworkStatus is the status for an Network resource.
type NetworkStatus struct {
	// DefaultStatus is a structure containing common fields used by all extension resources.
	DefaultStatus `json:",inline"`
}

// GetExtensionType returns the type of this Network resource.
func (n *Network) GetExtensionType() string {
	return n.Spec.Type
}
