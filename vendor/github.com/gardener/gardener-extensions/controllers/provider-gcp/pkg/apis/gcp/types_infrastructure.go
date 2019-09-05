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

package gcp

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureConfig infrastructure configuration resource
type InfrastructureConfig struct {
	metav1.TypeMeta

	// Networks is the network configuration (VPC, subnets, etc.)
	Networks NetworkConfig
}

// NetworkConfig holds information about the Kubernetes and infrastructure networks.
type NetworkConfig struct {
	// VPC indicates whether to use an existing VPC or create a new one.
	VPC *VPC
	// Internal is a private subnet (used for internal load balancers).
	Internal *string
	// Workers is the worker subnet range to create (used for the VMs).
	Worker string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureStatus contains information about created infrastructure resources.
type InfrastructureStatus struct {
	metav1.TypeMeta

	// Networks is the status of the networks of the infrastructure.
	Networks NetworkStatus

	// ServiceAccountEmail is the email address of the service account.
	ServiceAccountEmail string
}

// NetworkStatus is the current status of the infrastructure networks.
type NetworkStatus struct {
	// VPC states the name of the infrastructure VPC.
	VPC VPC

	// Subnets are the subnets that have been created.
	Subnets []Subnet
}

// SubnetPurpose is a purpose of a subnet.
type SubnetPurpose string

const (
	// PurposeNodes is a SubnetPurpose for nodes.
	PurposeNodes SubnetPurpose = "nodes"
	// PurposeInternal is a SubnetPurpose for internal use.
	PurposeInternal SubnetPurpose = "internal"
)

// Subnet is a subnet that was created.
type Subnet struct {
	// Name is the name of the subnet.
	Name string
	// Purpose is the purpose for which the subnet was created.
	Purpose SubnetPurpose
}

// VPC contains information about the VPC and some related resources.
type VPC struct {
	// Name is the VPC name.
	Name string
}
