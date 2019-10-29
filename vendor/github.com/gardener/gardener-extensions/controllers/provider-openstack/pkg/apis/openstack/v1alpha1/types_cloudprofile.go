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

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfileConfig contains provider-specific configuration that is embedded into Gardener's `CloudProfile`
// resource.
type CloudProfileConfig struct {
	metav1.TypeMeta `json:",inline"`
	// Constraints is an object containing constraints for certain values in the control plane config.
	Constraints Constraints `json:"constraints"`
	// DNSServers is a list of IPs of DNS servers used while creating subnets.
	// +optional
	DNSServers []string `json:"dnsServers,omitempty"`
	// DHCPDomain is the dhcp domain of the OpenStack system configured in nova.conf. Only meaningful for
	// Kubernetes 1.10.1+. See https://github.com/kubernetes/kubernetes/pull/61890 for details.
	// +optional
	DHCPDomain *string `json:"dhcpDomain,omitempty"`
	// KeyStoneURL is the URL for auth{n,z} in OpenStack (pointing to KeyStone).
	KeyStoneURL string `json:"keystoneURL"`
	// MachineImages is the list of machine images that are understood by the controller. It maps
	// logical names and versions to provider-specific identifiers.
	MachineImages []MachineImages `json:"machineImages"`
	// RequestTimeout specifies the HTTP timeout against the OpenStack API.
	// +optional
	RequestTimeout *string `json:"requestTimeout,omitempty"`
}

// Constraints is an object containing constraints for the shoots.
type Constraints struct {
	// FloatingPools contains constraints regarding allowed values of the 'floatingPoolName' block in the control plane config.
	FloatingPools []FloatingPool `json:"floatingPools"`
	// LoadBalancerProviders contains constraints regarding allowed values of the 'loadBalancerProvider' block in the control plane config.
	LoadBalancerProviders []LoadBalancerProvider `json:"loadBalancerProviders"`
}

// FloatingPool contains constraints regarding allowed values of the 'floatingPoolName' block in the control plane config.
type FloatingPool struct {
	// Name is the name of the floating pool.
	Name string `json:"name"`
	// LoadBalancerClasses contains a list of supported labeled load balancer network settings.
	// +optional
	LoadBalancerClasses []LoadBalancerClass `json:"loadBalancerClasses,omitempty"`
}

// LoadBalancerClass defines a restricted network setting for generic LoadBalancer classes.
type LoadBalancerClass struct {
	// Name is the name of the LB class
	Name string `json:"name"`
	// FloatingSubnetID is the subnetwork ID of a dedicated subnet in floating network pool.
	// +optional
	FloatingSubnetID *string `json:"floatingSubnetID,omitempty"`
	// FloatingNetworkID is the network ID of the floating network pool.
	// +optional
	FloatingNetworkID *string `json:"floatingNetworkID,omitempty"`
	// SubnetID is the ID of a local subnet used for LoadBalancer provisioning. Only usable if no FloatingPool
	// configuration is done.
	// +optional
	SubnetID *string `json:"subnetID,omitempty"`
}

// LoadBalancerProvider contains constraints regarding allowed values of the 'loadBalancerProvider' block in the control plane config.
type LoadBalancerProvider struct {
	// Name is the name of the load balancer provider.
	Name string `json:"name"`
}

// MachineImages is a mapping from logical names and versions to provider-specific identifiers.
type MachineImages struct {
	// Name is the logical name of the machine image.
	Name string `json:"name"`
	// Versions contains versions and a provider-specific identifier.
	Versions []MachineImageVersion `json:"versions"`
}

// MachineImageVersion contains a version and a provider-specific identifier.
type MachineImageVersion struct {
	// Version is the version of the image.
	Version string `json:"version"`
	// Image is the name of the image.
	Image string `json:"image"`
}
