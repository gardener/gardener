// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CredentialsBinding represents a binding to credentials in the same or another namespace.
type CredentialsBinding struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Provider defines the provider type of the CredentialsBinding.
	// This field is immutable.
	Provider CredentialsBindingProvider
	// Credentials specify reference to credentials.
	Credentials Credentials
	// Quotas is a list of references to Quota objects in the same or another namespace.
	// This field is immutable.
	Quotas []corev1.ObjectReference
}

// Credentials holds reference to credentials implementation.
type Credentials struct {
	// SecretRef is a reference to a secret object in the same or another namespace.
	SecretRef *corev1.SecretReference
	// WorkloadIdentityRef is a reference to a workloadidentity object in the same or another namespace.
	WorkloadIdentityRef *WorkloadIdentityReference
}

// GetProviderType gets the type of the provider.
func (cb *CredentialsBinding) GetProviderType() string {
	return cb.Provider.Type
}

// CredentialsBindingProvider defines the provider type of the CredentialsBinding.
type CredentialsBindingProvider struct {
	// Type is the type of the provider.
	Type string
}

// WorkloadIdentityReference represents a WorkloadIdentity Reference.
// It has enough information to retrieve workloadidentity in any namespace.
type WorkloadIdentityReference struct {
	// Name is unique within a namespace to reference a workloadidentity resource.
	Name string
	// Namespace defines the space within which the workloadidentity name must be unique.
	Namespace string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CredentialsBindingList is a collection of CredentialsBindings.
type CredentialsBindingList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of CredentialsBindings.
	Items []CredentialsBinding
}
