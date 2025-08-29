// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SecretBinding represents a binding to a secret in the same or another namespace.
//
// Deprecated: Use CredentialsBinding instead. See https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/secretbinding-to-credentialsbinding-migration.md for migration instructions.
type SecretBinding struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta

	// SecretRef is a reference to a secret object in the same or another namespace.
	// This field is immutable.
	SecretRef corev1.SecretReference
	// Quotas is a list of references to Quota objects in the same or another namespace.
	// This field is immutable.
	Quotas []corev1.ObjectReference
	// Provider defines the provider type of the SecretBinding.
	// This field is immutable.
	Provider *SecretBindingProvider
}

// SecretBindingProvider defines the provider type of the SecretBinding.
//
// Deprecated: Use CredentialsBindingProvider instead. See https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/secretbinding-to-credentialsbinding-migration.md for migration instructions.
type SecretBindingProvider struct {
	// Type is the type of the provider.
	//
	// For backwards compatibility, the field can contain multiple providers separated by a comma.
	// However the usage of single SecretBinding (hence Secret) for different cloud providers is strongly discouraged.
	Type string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SecretBindingList is a collection of SecretBindings.
//
// Deprecated: Use CredentialsBindingList instead. See https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/secretbinding-to-credentialsbinding-migration.md for migration instructions.
type SecretBindingList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta

	// Items is the list of SecretBindings.
	Items []SecretBinding
}
