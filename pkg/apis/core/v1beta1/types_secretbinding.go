// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SecretBinding struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// SecretRef is a reference to a secret object in the same or another namespace.
	SecretRef corev1.SecretReference `json:"secretRef" protobuf:"bytes,2,opt,name=secretRef"`
	// Quotas is a list of references to Quota objects in the same or another namespace.
	// +optional
	Quotas []corev1.ObjectReference `json:"quotas,omitempty" protobuf:"bytes,3,rep,name=quotas"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SecretBindingList is a collection of SecretBindings.
type SecretBindingList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of SecretBindings.
	Items []SecretBinding `json:"items" protobuf:"bytes,2,rep,name=items"`
}
