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

// ShootState contains a snapshot of the Shoot's state required to migrate the Shoot's control plane to a new Seed.
type ShootState struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the ShootState.
	// +optional
	Spec ShootStateSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootStateList is a list of ShootState objects.
type ShootStateList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of ShootStates.
	Items []ShootState `json:"items"`
}

// ShootStateSpec is the specification of the ShootState.
type ShootStateSpec struct {
	// Gardener holds the data required to generate resources deployed by the gardenlet
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	Gardener []GardenerResourceData `json:"gardener,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
	// Extensions holds the state of custom resources reconciled by extension controllers in the seed
	// +optional
	Extensions []ExtensionResourceState `json:"extensions,omitempty"`
}

// GardenerResourceData holds the data which is used to generate resources, deployed in the Shoot's control plane.
type GardenerResourceData struct {
	// Name of the object required to generate resources
	Name string `json:"name"`
	// Data contains the payload required to generate resources
	Data map[string]string `json:"data"`
}

// ExtensionResourceState contains the kind of the extension custom resource and its last observed state in the Shoot's
// namespace on the Seed cluster.
type ExtensionResourceState struct {
	// Kind (type) of the extension custom resource
	Kind string `json:"kind"`
	// Name of the extension custom resource
	// +optional
	Name *string `json:"name,omitempty"`
	// Purpose of the extension custom resource
	// +optional
	Purpose *string `json:"purpose,omitempty"`
	// State of the extension resource
	State ProviderConfig `json:"state"`
}
