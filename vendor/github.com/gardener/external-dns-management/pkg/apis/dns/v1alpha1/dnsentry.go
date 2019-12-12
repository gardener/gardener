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

type DNSEntryList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSEntry `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSEntry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DNSEntrySpec   `json:"spec"`
	Status            DNSEntryStatus `json:"status"`
}

type DNSEntrySpec struct {
	DNSName             string   `json:"dnsName"`
	OwnerId             *string  `json:"ownerId,omitempty"`
	TTL                 *int64   `json:"ttl,omitempty"`
	CNameLookupInterval *int64   `json:"cnameLookupInterval,omitempty"`
	Text                []string `json:"text,omitempty"`
	Targets             []string `json:"targets,omitempty"`
}

type DNSEntryStatus struct {
	ObservedGeneration int64    `json:"observedGeneration,omitempty"`
	State              string   `json:"state"`
	Message            *string  `json:"message,omitempty"`
	ProviderType       *string  `json:"providerType,omitempty"`
	Provider           *string  `json:"provider,omitempty"`
	Zone               *string  `json:"zone,omitempty"`
	TTL                *int64   `json:"ttl,omitempty"`
	Targets            []string `json:"targets,omitempty"`
}
