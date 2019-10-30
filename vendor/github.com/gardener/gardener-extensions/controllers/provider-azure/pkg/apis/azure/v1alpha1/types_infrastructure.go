// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

// InfrastructureConfig infrastructure configuration resource
type InfrastructureConfig struct {
	metav1.TypeMeta `json:",inline"`
	// ResourceGroup is azure resource group.
	// +optional
	ResourceGroup *ResourceGroup `json:"resourceGroup,omitempty"`
	// Networks is the network configuration (VNet, subnets, etc.).
	Networks NetworkConfig `json:"networks"`
	// Zoned indicates whether the cluster uses availability zones.
	// +optional
	Zoned bool `json:"zoned,omitempty"`
}

// ResourceGroup is azure resource group
type ResourceGroup struct {
	// Name is the name of the resource group
	Name string `json:"name"`
}

// NetworkConfig holds information about the Kubernetes and infrastructure networks.
type NetworkConfig struct {
	// VNet indicates whether to use an existing VNet or create a new one.
	VNet VNet `json:"vnet"`
	// Workers is the worker subnet range to create (used for the VMs).
	Workers string `json:"workers"`
	// ServiceEndpoints is a list of Azure ServiceEndpoints which should be associated with the worker subnet.
	// +optional
	ServiceEndpoints []string `json:"serviceEndpoints,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureStatus contains information about created infrastructure resources.
type InfrastructureStatus struct {
	metav1.TypeMeta `json:",inline"`
	// Networks is the status of the networks of the infrastructure.
	Networks NetworkStatus `json:"networks"`
	// ResourceGroup is azure resource group
	ResourceGroup ResourceGroup `json:"resourceGroup"`
	// AvailabilitySets is a list of created availability sets
	AvailabilitySets []AvailabilitySet `json:"availabilitySets"`
	// AvailabilitySets is a list of created route tables
	RouteTables []RouteTable `json:"routeTables"`
	// SecurityGroups is a list of created security groups
	SecurityGroups []SecurityGroup `json:"securityGroups"`
	// Zoned indicates whether the cluster uses zones
	// +optional
	Zoned bool `json:"zoned,omitempty"`
}

// NetworkStatus is the current status of the infrastructure networks.
type NetworkStatus struct {
	// VNetStatus states the name of the infrastructure VNet.
	VNet VNetStatus `json:"vnet"`

	// Subnets are the subnets that have been created.
	Subnets []Subnet `json:"subnets"`
}

// Purpose is a purpose of a subnet.
type Purpose string

const (
	// PurposeNodes is a Purpose for nodes.
	PurposeNodes Purpose = "nodes"
	// PurposeInternal is a Purpose for internal use.
	PurposeInternal Purpose = "internal"
)

// Subnet is a subnet that was created.
type Subnet struct {
	// Name is the name of the subnet.
	Name string `json:"name"`
	// Purpose is the purpose for which the subnet was created.
	Purpose Purpose `json:"purpose"`
}

// AvailabilitySet contains information about the azure availability set
type AvailabilitySet struct {
	// Purpose is the purpose of the availability set
	Purpose Purpose `json:"purpose"`
	// ID is the id of the availability set
	ID string `json:"id"`
	// Name is the name of the availability set
	Name string `json:"name"`
}

// RouteTable is the azure route table
type RouteTable struct {
	// Purpose is the purpose of the route table
	Purpose Purpose `json:"purpose"`
	// Name is the name of the route table
	Name string `json:"name"`
}

// SecurityGroup contains information about the security group
type SecurityGroup struct {
	// Purpose is the purpose of the security group
	Purpose Purpose `json:"purpose"`
	// Name is the name of the security group
	Name string `json:"name"`
}

// VNet contains information about the VNet and some related resources.
type VNet struct {
	// Name is the name of an existing vNet which should be used.
	// +optional
	Name *string `json:"name,omitempty"`
	// ResourceGroup is the resource group where the existing vNet blongs to.
	// +optional
	ResourceGroup *string `json:"resourceGroup,omitempty"`
	// CIDR is the VNet CIDR
	// +optional
	CIDR *string `json:"cidr,omitempty"`
}

// VNetStatus contains the VNet name.
type VNetStatus struct {
	// Name is the VNet name.
	Name string `json:"name"`
	// ResourceGroup is the resource group where the existing vNet belongs to.
	// +optional
	ResourceGroup *string `json:"resourceGroup,omitempty"`
}
