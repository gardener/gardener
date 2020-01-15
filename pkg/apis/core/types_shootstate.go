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

package core

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootState contains the state of a Shoot cluster required to migrate the Shoot's control plane to a new Seed.
type ShootState struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Specification of the ShootState.
	Spec ShootStateSpec
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootStateList is a list of ShootState objects.
type ShootStateList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of ShootStates.
	Items []ShootState
}

// ShootStateSpec is the specification of the ShootState.
type ShootStateSpec struct {
	// Gardener holds the data required to generate resources deployed by the gardenlet
	Gardener []GardenerResourceData
	// Extensions holds the state of custom resources reconciled by extension controllers in the seed
	Extensions []ExtensionResourceState
}

// GardenerResourceData holds the data which is used to generate resources, deployed in the Shoot's control plane.
type GardenerResourceData struct {
	// Name of the object required to generate resources
	Name string
	// Data contains the payload required to generate resources
	Data map[string]string
}

// ExtensionResourceState contains the kind of the extension custom resource and its last observed state in the Shoot's
// namespace on the Seed cluster.
type ExtensionResourceState struct {
	// Kind (type) of the extension custom resource
	Kind string
	// Name of the extension custom resource
	Name *string
	// Purpose of the extension custom resource
	Purpose *string
	// State of the extension resource
	State ProviderConfig
}
