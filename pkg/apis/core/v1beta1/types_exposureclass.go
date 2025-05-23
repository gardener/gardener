// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExposureClass represents a control plane endpoint exposure strategy.
type ExposureClass struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Handler is the name of the handler which applies the control plane endpoint exposure strategy.
	// This field is immutable.
	Handler string `json:"handler" protobuf:"bytes,2,opt,name=handler"`
	// Scheduling holds information how to select applicable Seed's for ExposureClass usage.
	// This field is immutable.
	// +optional
	Scheduling *ExposureClassScheduling `json:"scheduling,omitempty" protobuf:"bytes,3,opt,name=scheduling"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExposureClassList is a collection of ExposureClass.
type ExposureClassList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of ExposureClasses.
	Items []ExposureClass `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ExposureClassScheduling holds information to select applicable Seed's for ExposureClass usage.
type ExposureClassScheduling struct {
	// SeedSelector is an optional label selector for Seed's which are suitable to use the ExposureClass.
	// +optional
	SeedSelector *SeedSelector `json:"seedSelector,omitempty" protobuf:"bytes,1,opt,name=seedSelector"`
	// Tolerations contains the tolerations for taints on Seed clusters.
	// +patchMergeKey=key
	// +patchStrategy=merge
	// +optional
	Tolerations []Toleration `json:"tolerations,omitempty" patchStrategy:"merge" patchMergeKey:"key" protobuf:"bytes,2,rep,name=tolerations"`
}
