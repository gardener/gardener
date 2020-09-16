/*
 * SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSEntryList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSEntry `json:"items"`
}

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,path=dnsentries,shortName=dnse,singular=dnsentry
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=DNS,description="FQDN of DNS Entry",JSONPath=".spec.dnsName",type=string
// +kubebuilder:printcolumn:name=OWNERID,JSONPath=".spec.ownerId",type=string
// +kubebuilder:printcolumn:name=TYPE,JSONPath=".status.providerType",type=string
// +kubebuilder:printcolumn:name=PROVIDER,JSONPath=".status.provider",type=string
// +kubebuilder:printcolumn:name=STATUS,JSONPath=".status.state",type=string
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSEntry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DNSEntrySpec `json:"spec"`
	// +optional
	Status DNSEntryStatus `json:"status,omitempty"`
}

type DNSEntrySpec struct {
	// full qualified domain name
	DNSName string `json:"dnsName"`
	// reference to base entry used to inherit attributes from
	// +optional
	Reference *EntryReference `json:"reference,omitempty"`
	// owner id used to tag entries in external DNS system
	// +optional
	OwnerId *string `json:"ownerId,omitempty"`
	// time to live for records in external DNS system
	// +optional
	TTL *int64 `json:"ttl,omitempty"`
	// lookup interval for CNAMEs that must be resolved to IP addresses
	// +optional
	CNameLookupInterval *int64 `json:"cnameLookupInterval,omitempty"`
	// text records, either text or targets must be specified
	// +optional
	Text []string `json:"text,omitempty"`
	// target records (CNAME or A records), either text or targets must be specified
	// +optional
	Targets []string `json:"targets,omitempty"`
}

type DNSEntryStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// entry state
	// +optional
	State string `json:"state"`
	// message describing the reason for the state
	// +optional
	Message *string `json:"message,omitempty"`
	// provider type used for the entry
	// +optional
	ProviderType *string `json:"providerType,omitempty"`
	// assigned provider
	// +optional
	Provider *string `json:"provider,omitempty"`
	// zone used for the entry
	// +optional
	Zone *string `json:"zone,omitempty"`
	// time to live used for the entry
	// +optional
	TTL *int64 `json:"ttl,omitempty"`
	// effective targets generated for the entry
	// +optional
	Targets []string `json:"targets,omitempty"`
}

type EntryReference struct {
	// name of the referenced DNSEntry object
	Name string `json:"name"`
	// namespace of the referenced DNSEntry object
	// +optional
	Namespace string `json:"namespace,omitempty"`
}
