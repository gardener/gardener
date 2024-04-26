// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	// Credentials specify reference to credentials.
	Credentials Credentials `json:"credentials" protobuf:"bytes,3,name=credentials"`
	// Quotas is a list of references to Quota objects in the same or another namespace.
	// This field is immutable.
	// +optional
	Quotas []corev1.ObjectReference `json:"quotas,omitempty" protobuf:"bytes,4,rep,name=quotas"`
}

// Credentials holds reference to credentials implementation.
type Credentials struct {
	// SecretRef is a reference to a secret object in the same or another namespace.
	// +optional
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty" protobuf:"bytes,1,opt,name=secretRef"`
	// WorkloadIdentityRef is a reference to a workloadidentity object in the same or another namespace.
	// +optional
	WorkloadIdentityRef *WorkloadIdentityReference `json:"workloadIdentityRef,omitempty" protobuf:"bytes,2,opt,name=workloadIdentityRef"`
}

// GetProviderType gets the type of the provider.
func (cb *CredentialsBinding) GetProviderType() string {
	return cb.Provider.Type
}

// CredentialsBindingProvider defines the provider type of the CredentialsBinding.
type CredentialsBindingProvider struct {
	// Type is the type of the provider.
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
}

// WorkloadIdentityReference represents a WorkloadIdentity Reference.
// It has enough information to retrieve workloadidentity in any namespace.
type WorkloadIdentityReference struct {
	// Name is unique within a namespace to reference a workloadidentity resource.
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Namespace defines the space within which the workloadidentity name must be unique.
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
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
