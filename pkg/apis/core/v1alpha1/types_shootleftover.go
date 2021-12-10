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

package v1alpha1

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootLeftover represents leftover resources in a Seed cluster that once contained the control plane
// of a Shoot cluster that are no longer needed and should be properly cleaned up.
type ShootLeftover struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Spec defines the ShootLeftover properties.
	// +optional
	Spec ShootLeftoverSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// Most recently observed status of the ShootLeftover.
	// +optional
	Status ShootLeftoverStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootLeftoverList is a collection of ShootLeftovers.
type ShootLeftoverList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of ShootLeftovers.
	Items []ShootLeftover `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ShootLeftoverSpec is the specification of a ShootLeftover.
type ShootLeftoverSpec struct {
	// SeedName is the name of the Seed cluster that contains the leftover resources.
	SeedName string `json:"seedName" protobuf:"bytes,1,opt,name=seedName"`
	// ShootName is the name of the Shoot cluster.
	ShootName string `json:"shootName" protobuf:"bytes,2,opt,name=shootName"`
	// TechnicalID is the technical ID of the Shoot cluster.
	// It is the name of the leftover namespace and Cluster resource, and part of the name of the BackupEntry resource.
	// If nil, it will be determined automatically.
	// +optional
	TechnicalID *string `json:"technicalID,omitempty" protobuf:"bytes,3,opt,name=technicalID"`
	// UID is the unique identifier of the Shoot cluster. It is part of the name of the BackupEntry resource.
	// If nil, it will be determined automatically if the shoot still exists.
	// +optional
	UID *types.UID `json:"uid,omitempty" protobuf:"bytes,4,opt,name=uid,casttype=k8s.io/apimachinery/pkg/types.UID"`
}

// ShootLeftoverStatus holds the most recently observed status of the ShootLeftover.
type ShootLeftoverStatus struct {
	// Conditions represents the latest available observations of a ShootLeftover's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// LastOperation holds information about the last operation on the ShootLeftover.
	// +optional
	LastOperation *gardencorev1beta1.LastOperation `json:"lastOperation,omitempty" protobuf:"bytes,2,opt,name=lastOperation"`
	// LastErrors holds information about the last occurred error(s) during an operation.
	// +optional
	LastErrors []gardencorev1beta1.LastError `json:"lastErrors,omitempty" protobuf:"bytes,3,rep,name=lastErrors"`
	// ObservedGeneration is the most recent generation observed for this ShootLeftover.
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,4,opt,name=observedGeneration"`
}

const (
	// ShootLeftoverResourcesExist is a constant for a condition type indicating that some leftover resources still exist in the Seed cluster.
	ShootLeftoverResourcesExist ConditionType = "ResourcesExist"
)
