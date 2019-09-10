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

package alicloud

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureConfig infrastructure configuration resource
type InfrastructureConfig struct {
	metav1.TypeMeta

	// Networks specifies the networks for an infrastructure.
	Networks Networks
}

// Networks specifies the networks for an infrastructure.
type Networks struct {
	// VPC contains information about whether to create a new or use an existing VPC.
	VPC VPC

	// Zones are the network zones for an infrastructure.
	Zones []Zone
}

// VPC contains information about whether to create a new or use an existing VPC.
type VPC struct {
	// ID is the ID of an existing VPC.
	// +optional
	ID *string
	// CIDR is the CIDR of a VPC to create.
	// +optional
	CIDR *string
}

// VPCStatus contains output information about the VPC.
type VPCStatus struct {
	// ID is the ID of the VPC.
	ID string
	// VSwitches is a list of vswitches.
	VSwitches []VSwitch
	// SecurityGroups is a list of security groups.
	SecurityGroups []SecurityGroup
}

// Purpose is a purpose of a subnet.
type Purpose string

const (
	// PurposeNodes is a Purpose for nodes.
	PurposeNodes Purpose = "nodes"
	// PurposeInternal is a Purpose for internal use.
	PurposeInternal Purpose = "internal"
)

// VSwitch contains information about a vswitch.
type VSwitch struct {
	// Purpose is the purpose for which the vswitch was created.
	Purpose Purpose
	// ID is the id of the vswitch.
	ID string
	// Zone is the name of the zone.
	Zone string
}

// SecurityGroup contains information about a security group.
type SecurityGroup struct {
	// Purpose is the purpose for which the security group was created.
	Purpose Purpose
	// ID is the id of the security group.
	ID string
}

// Zone is a zone with a name and worker CIDR.
type Zone struct {
	// Name is the name of a zone.
	Name string
	// Worker specifies the worker CIDR to use.
	Worker string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureStatus contains information about created infrastructure resources.
type InfrastructureStatus struct {
	metav1.TypeMeta

	VPC         VPCStatus
	KeyPairName string
}
