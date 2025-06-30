// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootState contains a snapshot of the Shoot's state required to migrate the Shoot's control plane to a new Seed.
type ShootState struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the ShootState.
	// +optional
	Spec ShootStateSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ShootStateList is a list of ShootState objects.
type ShootStateList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is the list of ShootStates.
	Items []ShootState `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ShootStateSpec is the specification of the ShootState.
type ShootStateSpec struct {
	// Gardener holds the data required to generate resources deployed by the gardenlet
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	Gardener []GardenerResourceData `json:"gardener,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,1,rep,name=gardener"`
	// Extensions holds the state of custom resources reconciled by extension controllers in the seed
	// +optional
	Extensions []ExtensionResourceState `json:"extensions,omitempty" protobuf:"bytes,2,rep,name=extensions"`
	// Resources holds the data of resources referred to by extension controller states
	// +optional
	Resources []ResourceData `json:"resources,omitempty" protobuf:"bytes,3,rep,name=resources"`
}

// GardenerResourceData holds the data which is used to generate resources, deployed in the Shoot's control plane.
type GardenerResourceData struct {
	// Name of the object required to generate resources
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Type of the object
	Type string `json:"type" protobuf:"bytes,2,opt,name=type"`
	// Data contains the payload required to generate resources
	Data runtime.RawExtension `json:"data" protobuf:"bytes,3,opt,name=data"`
	// Labels are labels of the object
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,4,opt,name=labels"`
}

// ExtensionResourceState contains the kind of the extension custom resource and its last observed state in the Shoot's
// namespace on the Seed cluster.
type ExtensionResourceState struct {
	// Kind (type) of the extension custom resource
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	// Name of the extension custom resource
	// +optional
	Name *string `json:"name,omitempty" protobuf:"bytes,2,opt,name=name"`
	// Purpose of the extension custom resource
	// +optional
	Purpose *string `json:"purpose,omitempty" protobuf:"bytes,3,opt,name=purpose"`
	// State of the extension resource
	// +optional
	State *runtime.RawExtension `json:"state,omitempty" protobuf:"bytes,4,opt,name=state"`
	// Resources holds a list of named resource references that can be referred to in the state by their names.
	// +optional
	Resources []NamedResourceReference `json:"resources,omitempty" protobuf:"bytes,5,rep,name=resources"`
}

// ResourceData holds the data of a resource referred to by an extension controller state.
type ResourceData struct {
	autoscalingv1.CrossVersionObjectReference `json:",inline" protobuf:"bytes,1,opt,name=ref"`

	// Data of the resource
	Data runtime.RawExtension `json:"data" protobuf:"bytes,2,opt,name=data"`
}
