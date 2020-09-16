/*
 * SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
 *
 * SPDX-License-Identifier: Apache-2.0
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

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,path=dnsproviders,shortName=dnspr,singular=dnsprovider
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=TYPE,JSONPath=".spec.type",type=string
// +kubebuilder:printcolumn:name=STATUS,JSONPath=".status.state",type=string
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DNSProviderSpec `json:"spec"`
	// +optional
	Status DNSProviderStatus `json:"status,omitempty"`
}

type DNSProviderSpec struct {
	// type of the provider (selecting the responsible type of DNS controller)
	Type string `json:"type,omitempty"`
	// optional additional provider specific configuration values
	// +kubebuilder:validation:XPreserveUnknownFields
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty"`
	// access credential for the external DNS system of the given type
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty"`
	// desired selection of usable domains
	// (by default all zones and domains in those zones will be served)
	// +optional
	Domains *DNSSelection `json:"domains,omitempty"`
	// desired selection of usable domains
	// the domain selection is used for served zones, only
	// (by default all zones will be served)
	// +optional
	Zones *DNSSelection `json:"zones,omitempty"`
}

type DNSSelection struct {
	// values that should be observed (domains or zones)
	// + optional
	Include []string `json:"include,omitempty"`
	// values that should be ignored (domains or zones)
	// + optional
	Exclude []string `json:"exclude,omitempty"`
}

type DNSProviderStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// state of the provider
	// +optional
	State string `json:"state"`
	// message describing the reason for the actual state of the provider
	Message *string `json:"message,omitempty"`
	// actually served domain selection
	// +optional
	Domains DNSSelectionStatus `json:"domains"`
	// actually served zones
	// +optional
	Zones DNSSelectionStatus `json:"zones"`
}

type DNSSelectionStatus struct {
	// included values (domains or zones)
	// + optional
	Included []string `json:"included,omitempty"`
	// Excluded values (domains or zones)
	// + optional
	Excluded []string `json:"excluded,omitempty"`
}
