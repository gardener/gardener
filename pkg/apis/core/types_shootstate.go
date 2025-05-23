// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

import (
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	// Resources holds the data of resources referred to by extension controller states
	Resources []ResourceData
}

// GardenerResourceData holds the data which is used to generate resources, deployed in the Shoot's control plane.
type GardenerResourceData struct {
	// Name of the object required to generate resources
	Name string
	// Type of the object
	Type string
	// Data contains the payload required to generate resources
	Data runtime.RawExtension
	// Labels are labels of the object
	Labels map[string]string
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
	State *runtime.RawExtension
	// Resources holds a list of named resource references that can be referred to in the state by their names.
	Resources []NamedResourceReference
}

// ResourceData holds the data of a resource referred to by an extension controller state.
type ResourceData struct {
	autoscalingv1.CrossVersionObjectReference
	// Data of the resource
	Data runtime.RawExtension
}
