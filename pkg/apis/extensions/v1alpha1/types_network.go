// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
