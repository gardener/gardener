/*
 * Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 *
 */

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSProviderList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSProvider `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DNSProviderSpec   `json:"spec"`
	Status            DNSProviderStatus `json:"status"`
}

type DNSProviderSpec struct {
	Type           string                  `json:"type,omitempty"`
	ProviderConfig *runtime.RawExtension   `json:"providerConfig,omitempty"`
	SecretRef      *corev1.SecretReference `json:"secretRef,omitempty"`
	Domains        *DNSSelection           `json:"domains,omitempty"`
	Zones          *DNSSelection           `json:"zones,omitempty"`
}

type DNSSelection struct {
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

type DNSProviderStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	State              string             `json:"state"`
	Message            *string            `json:"message,omitempty"`
	Domains            DNSSelectionStatus `json:"domains"`
	Zones              DNSSelectionStatus `json:"zones"`
}

type DNSSelectionStatus struct {
	Included []string `json:"included,omitempty"`
	Excluded []string `json:"excluded,omitempty"`
}
