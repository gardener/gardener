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

package azure

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureConfig infrastructure configuration resource
type InfrastructureConfig struct {
	metav1.TypeMeta
	// ResourceGroup is azure resource group
	ResourceGroup *ResourceGroup
	// Networks is the network configuration (VNets, subnets, etc.)
	Networks NetworkConfig
	// Zoned indicates whether the cluster uses zones
	Zoned bool
}

// ResourceGroup is azure resource group
type ResourceGroup struct {
	// Name is the name of the resource group
	Name string
}

// NetworkConfig holds information about the Kubernetes and infrastructure networks.
type NetworkConfig struct {
	// VNet indicates whether to use an existing VNet or create a new one.
	VNet VNet
	// Workers is the worker subnet range to create (used for the VMs).
	Workers string
	// ServiceEndpoints is a list of Azure ServiceEndpoints which should be associated with the worker subnet.
	ServiceEndpoints []string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureStatus contains information about created infrastructure resources.
type InfrastructureStatus struct {
	metav1.TypeMeta
	// Networks is the status of the networks of the infrastructure.
	Networks NetworkStatus
	// ResourceGroup is azure resource group
	ResourceGroup ResourceGroup
	// AvailabilitySets is a list of created availability sets
	AvailabilitySets []AvailabilitySet
	// AvailabilitySets is a list of created route tables
	RouteTables []RouteTable
	// SecurityGroups is a list of created security groups
	SecurityGroups []SecurityGroup
	// Zoned indicates whether the cluster uses zones
	Zoned bool
}

// NetworkStatus is the current status of the infrastructure networks.
type NetworkStatus struct {
	// VNet states the name of the infrastructure VNet.
	VNet VNetStatus
	// Subnets are the subnets that have been created.
	Subnets []Subnet
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
	Name string
	// Purpose is the purpose for which the subnet was created.
	Purpose Purpose
}

// AvailabilitySet contains information about the azure availability set
type AvailabilitySet struct {
	// Purpose is the purpose of the availability set
	Purpose Purpose
	// ID is the id of the availability set
	ID string
	// Name is the name of the availability set
	Name string
}

// RouteTable is the azure route table
type RouteTable struct {
	// Purpose is the purpose of the route table
	Purpose Purpose
	// Name is the name of the route table
	Name string
}

// SecurityGroup contains information about the security group
type SecurityGroup struct {
	// Purpose is the purpose of the security group
	Purpose Purpose
	// Name is the name of the security group
	Name string
}

// VNet contains information about the VNet and some related resources.
type VNet struct {
	// Name is the VNet name.
	Name *string
	// ResourceGroup is the resource group where the existing vNet belongs to.
	ResourceGroup *string
	// CIDR is the VNet CIDR
	CIDR *string
}

// VNetStatus contains the VNet name.
type VNetStatus struct {
	// Name is the VNet name.
	Name string
	// ResourceGroup is the resource group where the existing vNet belongs to.
	ResourceGroup *string
}
