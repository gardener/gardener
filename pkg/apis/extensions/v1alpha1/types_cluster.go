// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ClusterResource is a constant for the name of the Cluster resource.
const ClusterResource = "Cluster"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Cluster is a specification for a Cluster resource.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterSpec `json:"spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterList is a list of Cluster resources.
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// Items is the list of Cluster.
	Items []Cluster `json:"items"`
}

// ClusterSpec is the spec for a Cluster resource.
type ClusterSpec struct {
	// CloudProfile is a raw extension field that contains the cloudprofile resource referenced
	// by the shoot that has to be reconciled.
	CloudProfile runtime.RawExtension `json:"cloudProfile"`
	// Seed is a raw extension field that contains the seed resource referenced by the shoot that
	// has to be reconciled.
	Seed runtime.RawExtension `json:"seed"`
	// Shoot is a raw extension field that contains the shoot resource that has to be reconciled.
	Shoot runtime.RawExtension `json:"shoot"`
}
