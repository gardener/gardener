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

type DNSOwnerList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSOwner `json:"items"`
}

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,path=dnsowners,shortName=dnso,singular=dnsowner
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=OwnerId,JSONPath=".spec.ownerId",type=string
// +kubebuilder:printcolumn:name=Active,JSONPath=".spec.active",type=boolean
// +kubebuilder:printcolumn:name=Usages,JSONPath=".status.amount",type=string
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSOwner struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DNSOwnerSpec `json:"spec"`
	// +optional
	Status DNSOwnerStatus `json:"status,omitempty"`
}

type DNSOwnerSpec struct {
	// owner id used to tag entries in external DNS system
	OwnerId string `json:"ownerId"`
	// state of the ownerid for the DNS controller observing entry using this owner id
	// (default:true)
	// +optional
	Active *bool `json:"active,omitempty"`
}

type DNSOwnerStatus struct {
	// Entry statistic for this owner id
	// +optional
	Entries DNSOwnerStatusEntries `json:"entries,omitempty"`
}

type DNSOwnerStatusEntries struct {
	// number of entries using this owner id
	// +optional
	Amount int `json:"amount"`
	// number of entries per provider type
	// +optional
	ByType map[string]int `json:"types,omitempty"`
}
