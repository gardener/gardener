// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SecretBinding represents a binding to a secret in the same or another namespace.
type SecretBinding struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// SecretRef is a reference to a secret object in the same or another namespace.
	// This field is immutable.
	SecretRef corev1.SecretReference `json:"secretRef" protobuf:"bytes,2,opt,name=secretRef"`
	// Quotas is a list of references to Quota objects in the same or another namespace.
	// This field is immutable.
	// +optional
	Quotas []corev1.ObjectReference `json:"quotas,omitempty" protobuf:"bytes,3,rep,name=quotas"`
	// Provider defines the provider type of the SecretBinding.
	// This field is immutable.
	// +optional
	Provider *SecretBindingProvider `json:"provider,omitempty" protobuf:"bytes,4,opt,name=provider"`
}

// SecretBindingProvider defines the provider type of the SecretBinding.
type SecretBindingProvider struct {
	// Type is the type of the provider.
	//
	// For backwards compatibility, the field can contain multiple providers separated by a comma.
	// However the usage of single SecretBinding (hence Secret) for different cloud providers is strongly discouraged.
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
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
