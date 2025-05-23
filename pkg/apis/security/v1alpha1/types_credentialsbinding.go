// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CredentialsBinding represents a binding to credentials in the same or another namespace.
type CredentialsBinding struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Provider defines the provider type of the CredentialsBinding.
	// This field is immutable.
	Provider CredentialsBindingProvider `json:"provider" protobuf:"bytes,2,opt,name=provider"`
	// CredentialsRef is a reference to a resource holding the credentials.
	// Accepted resources are core/v1.Secret and security.gardener.cloud/v1alpha1.WorkloadIdentity
	// This field is immutable.
	CredentialsRef corev1.ObjectReference `json:"credentialsRef" protobuf:"bytes,3,name=credentialsRef"`
	// Quotas is a list of references to Quota objects in the same or another namespace.
	// This field is immutable.
	// +optional
	Quotas []corev1.ObjectReference `json:"quotas,omitempty" protobuf:"bytes,4,rep,name=quotas"`
}

// CredentialsBindingProvider defines the provider type of the CredentialsBinding.
type CredentialsBindingProvider struct {
	// Type is the type of the provider.
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CredentialsBindingList is a collection of CredentialsBindings.
type CredentialsBindingList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of CredentialsBindings.
	Items []CredentialsBinding `json:"items" protobuf:"bytes,2,rep,name=items"`
}
