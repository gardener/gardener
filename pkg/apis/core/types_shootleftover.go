// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package core

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootLeftover represents leftover resources in a Seed cluster that once contained the control plane
// of a Shoot cluster that are no longer needed and should be properly cleaned up.
type ShootLeftover struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec defines the ShootLeftover properties.
	Spec ShootLeftoverSpec
	// Most recently observed status of the ShootLeftover.
	Status ShootLeftoverStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootLeftoverList is a collection of ShootLeftovers.
type ShootLeftoverList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of ShootLeftovers.
	Items []ShootLeftover
}

// ShootLeftoverSpec is the specification of an ShootLeftover.
type ShootLeftoverSpec struct {
	// SeedName is the name of the Seed cluster that contains the leftover resources.
	SeedName string
	// ShootName is the name of the Shoot cluster.
	ShootName string
	// TechnicalID is the technical ID of the Shoot cluster.
	// It is the name of the leftover namespace and Cluster resource, and part of the name of the BackupEntry resource.
	// If nil, it will be determined automatically.
	TechnicalID *string
	// UID is the unique identifier of the Shoot cluster. It is part of the name of the BackupEntry resource.
	// If nil, it will be determined automatically if the shoot still exists.
	UID *types.UID
}

// ShootLeftoverStatus holds the most recently observed status of the ShootLeftover.
type ShootLeftoverStatus struct {
	// Conditions represents the latest available observations of a ShootLeftover's current state.
	Conditions []Condition
	// LastOperation holds information about the last operation on the ShootLeftover.
	LastOperation *LastOperation
	// LastErrors holds information about the last occurred error(s) during an operation.
	LastErrors []LastError
	// ObservedGeneration is the most recent generation observed for this ShootLeftover.
	ObservedGeneration int64
}

const (
	// ShootLeftoverResourcesExist is a constant for a condition type indicating that some leftover resources still exist in the Seed cluster.
	ShootLeftoverResourcesExist ConditionType = "ResourcesExist"
)
