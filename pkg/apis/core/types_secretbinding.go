// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SecretBinding struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// SecretRef is a reference to a secret object in the same or another namespace.
	SecretRef corev1.SecretReference
	// Quotas is a list of references to Quota objects in the same or another namespace.
	Quotas []corev1.ObjectReference
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SecretBindingList is a collection of SecretBindings.
type SecretBindingList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of SecretBindings.
	Items []SecretBinding
}
