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
// +kubebuilder:printcolumn:name=Active,JSONPath=".status.active",type=boolean
// +kubebuilder:printcolumn:name=Usages,JSONPath=".status.entries.amount",type=integer
// +kubebuilder:printcolumn:name=Valid,JSONPath=".spec.validUntil",type=string,format=date-time,description="expiration date"
// +kubebuilder:printcolumn:name=AGE,JSONPath=".metadata.creationTimestamp",type=date,description="creation timestamp"
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
	// Optional activation info for controlling the owner activation remotely via DNS TXT record
	// +optional
	DNSActivation *DNSActivation `json:"dnsActivation,omitempty"`

	// +optional
	// optional time this owner should be active if active flag is not false
	ValidUntil *metav1.Time `json:"validUntil,omitempty"`
}

// DNSActivation carries the optinal informatio required to control the
// activation of an owner id remotely via TXT records
type DNSActivation struct {
	// DNS name for controlling the owner activation remotely via DNS TXT record
	DNSName string `json:"dnsName"`
	// Optional value for the DNS activation record used to activate this owner
	// The default is the id of the cluster used to read the owner object
	// +optional
	Value *string `json:"value,omitempty"`
}

type DNSOwnerStatus struct {
	// state of the ownerid for the DNS controller observing entry using this owner id
	// +optional
	Active *bool `json:"active,omitempty"`
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
