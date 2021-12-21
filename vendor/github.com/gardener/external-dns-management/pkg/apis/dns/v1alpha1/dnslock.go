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

type DNSLockList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSLock `json:"items"`
}

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,path=dnslocks,shortName=dnsl,singular=dnslock
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=DNS,description="FQDN of DNS Entry",JSONPath=".spec.dnsName",type=string
// +kubebuilder:printcolumn:name=TYPE,JSONPath=".status.providerType",type=string,description="provider type"
// +kubebuilder:printcolumn:name=PROVIDER,JSONPath=".status.provider",type=string,description="assigned provider (namespace/name)"
// +kubebuilder:printcolumn:name=STATUS,JSONPath=".status.state",type=string,description="entry status"
// +kubebuilder:printcolumn:name=AGE,JSONPath=".metadata.creationTimestamp",type=date,description="entry creation timestamp"
// +kubebuilder:printcolumn:name=OWNERID,JSONPath=".spec.ownerGroupId",type=string,description="owner group id used to tag entries in external DNS system"
// +kubebuilder:printcolumn:name=TTL,JSONPath=".status.ttl",type=integer,priority=2000,description="time to live"
// +kubebuilder:printcolumn:name=ZONE,JSONPath=".status.zone",type=string,priority=2000,description="zone id"
// +kubebuilder:printcolumn:name=MESSAGE,JSONPath=".status.message",type=string,priority=2000,description="message describing the reason for the state"
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSLock struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DNSLockSpec `json:"spec"`
	// +optional
	Status DNSLockStatus `json:"status,omitempty"`
}

type DNSLockSpec struct {
	// full qualified domain name
	DNSName string `json:"dnsName"`
	// owner group for collaboration of multiple controller
	// +optional
	LockId *string `json:"lockId,omitempty"`
	// time to live for records in external DNS system
	TTL int64 `json:"ttl"`
	// Activation time stamp
	Timestamp metav1.Time `json:"timestamp"`
	// attribute values (must be compatible with DNS TXT records)
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`
}

type DNSLockStatus struct {
	DNSBaseStatus `json:",inline"`
	// Activation time stamp found in DNS
	// +optional
	Timestamp *metav1.Time `json:"timestamp,omitempty"`
	// owner group for collaboration of multiple controller found in DNS
	// +optional
	LockId *string `json:"lockId,omitempty"`
	// attribute values found in DNS
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`

	// First failed DNS looup
	// +optional
	FirstFailedDNSLookup *metav1.Time `json:"firstFailedDNSLookup,omitempty"`
}
