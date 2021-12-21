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

type DNSHostedZonePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSHostedZonePolicy `json:"items"`
}

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,path=dnshostedzonepolicies,shortName=dnshzp,singular=dnshostedzonepolicy
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Zone Count,JSONPath=".status.count",type=integer
// +kubebuilder:printcolumn:name=Age,JSONPath=".metadata.creationTimestamp",type=date
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSHostedZonePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DNSHostedZonePolicySpec `json:"spec"`
	// +optional
	Status DNSHostedZonePolicyStatus `json:"status,omitempty"`
}

type DNSHostedZonePolicySpec struct {
	// ZoneSelector specifies the selector for the DNS hosted zones
	Selector ZoneSelector `json:"selector"`
	Policy   ZonePolicy   `json:"policy"`
}

// ZoneSelector selects by intersection
type ZoneSelector struct {
	// DomainNames selects by base domain name of hosted zone.
	// Policy will be applied to zones with matching base domain
	// +optional
	DomainNames []string `json:"domainNames,omitempty"`
	// ProviderTypes selects by provider types
	// +optional
	ProviderTypes []string `json:"providerTypes,omitempty"`
	// ZoneIDs selects by provider dependent zone ID
	// +optional
	ZoneIDs []string `json:"zoneIDs,omitempty"`
}

// ZonePolicy specifies zone specific policy
type ZonePolicy struct {
	// ZoneStateCacheTTL specifies the TTL for the zone state cache
	// +optional
	ZoneStateCacheTTL *metav1.Duration `json:"zoneStateCacheTTL,omitempty"`
}

type DNSHostedZonePolicyStatus struct {
	// Number of zones this policy is applied to
	// +optional
	Count *int `json:"count,omitempty"`
	// Indicates that annotation is observed by a DNS sorce controller
	// +optional
	Zones []ZoneInfo `json:"zones,omitempty"`
	// LastStatusUpdateTime contains the timestamp of the last status update
	// +optional
	LastStatusUpdateTime *metav1.Time `json:"lastStatusUpdateTime,omitempty"`
	// In case of a configuration problem this field describes the reason
	// +optional
	Message *string `json:"message,omitempty"`
}

type ZoneInfo struct {
	// ID of the zone
	ZoneID string `json:"zoneID"`
	// Provider type of the zone
	ProviderType string `json:"providerType"`
	// Domain name of the zone
	DomainName string `json:"domainName"`
}
