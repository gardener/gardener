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

// InternalSecret holds secret data of a certain type.  The total bytes of the values in
// the Data field must be less than MaxSecretSize bytes.
type InternalSecret struct {
	metav1.TypeMeta
	// +optional
	metav1.ObjectMeta

	// Immutable field, if set, ensures that data stored in the Secret cannot
	// be updated (only object metadata can be modified).
	// +optional
	Immutable *bool

	// Data contains the secret data. Each key must consist of alphanumeric
	// characters, '-', '_' or '.'. The serialized form of the secret data is a
	// base64 encoded string, representing the arbitrary (possibly non-string)
	// data value here.
	// +optional
	Data map[string][]byte `datapolicy:"password,security-key,token"`

	// Used to facilitate programmatic handling of secret data.
	// More info: https://kubernetes.io/docs/concepts/configuration/secret/#secret-types
	// +optional
	Type corev1.SecretType
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InternalSecretList represents the list of secrets
type InternalSecretList struct {
	metav1.TypeMeta
	// +optional
	metav1.ListMeta

	Items []InternalSecret
}
