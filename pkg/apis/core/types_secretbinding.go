// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package core

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SecretBinding represents a binding to a secret in the same or another namespace.
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

// GetProviderType gets the type of the provider.
func (sb *SecretBinding) GetProviderType() string {
	if sb.Provider == nil {
		return ""
	}

	return sb.Provider.Type
}

// SecretBindingProvider defines the provider type of the SecretBinding.
type SecretBindingProvider struct {
	// Type is the type of the provider.
	//
	// For backwards compatibility, the field can contain multiple providers separated by a comma.
	// However the usage of single SecretBinding (hence Secret) for different cloud providers is strongly discouraged.
	Type string
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
