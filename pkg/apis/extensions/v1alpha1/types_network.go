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
	"k8s.io/apimachinery/pkg/runtime"
)

var _ Object = (*Network)(nil)

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
	// ProviderConfig contains plugin-specific configuration.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty"`
}

// NetworkStatus is the status for an Network resource.
type NetworkStatus struct {
	// DefaultStatus is a structure containing common fields used by all extension resources.
	DefaultStatus `json:",inline"`

	// ProviderStatus contains plugin-specific output.
	// +optional
	ProviderStatus *runtime.RawExtension `json:"providerStatus,omitempty"`
}

// GetExtensionType returns the type of this Network resource.
func (n *Network) GetExtensionType() string {
	return n.Spec.Type
}
