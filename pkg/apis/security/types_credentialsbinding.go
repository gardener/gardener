// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package security

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
	// CredentialsRef is a reference to a resource holding the credentials.
	// Accepted resources are core/v1.Secret and security.gardener.cloud/v1alpha1.WorkloadIdentity
	// This field is immutable.
	CredentialsRef corev1.ObjectReference
	// Quotas is a list of references to Quota objects in the same or another namespace.
	// This field is immutable.
	Quotas []corev1.ObjectReference
}

// CredentialsBindingProvider defines the provider type of the CredentialsBinding.
type CredentialsBindingProvider struct {
	// Type is the type of the provider.
	Type string
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
