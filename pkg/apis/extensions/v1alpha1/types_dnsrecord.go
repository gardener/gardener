// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ Object = (*DNSRecord)(nil)

// DNSRecordResource is a constant for the name of the DNSRecord resource.
const DNSRecordResource = "DNSRecord"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,path=dnsrecords,shortName=dns,singular=dnsrecord
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Type,JSONPath=".spec.type",type=string,description="The DNS record provider type."
// +kubebuilder:printcolumn:name="Domain Name",JSONPath=".spec.name",type=string,description="The DNS record domain name."
// +kubebuilder:printcolumn:name="Record Type",JSONPath=".spec.recordType",type=string,description="The DNS record type (A, CNAME, or TXT)."
// +kubebuilder:printcolumn:name=Status,JSONPath=".status.lastOperation.state",type=string,description=""
// +kubebuilder:printcolumn:name=Age,JSONPath=".metadata.creationTimestamp",type=date,description=""

// DNSRecord is a specification for a DNSRecord resource.
type DNSRecord struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the DNSRecord.
	// If the object's deletion timestamp is set, this field is immutable.
	Spec DNSRecordSpec `json:"spec"`
	// +optional
	Status DNSRecordStatus `json:"status"`
}

// GetExtensionSpec implements Object.
func (i *DNSRecord) GetExtensionSpec() Spec {
	return &i.Spec
}

// GetExtensionStatus implements Object.
func (i *DNSRecord) GetExtensionStatus() Status {
	return &i.Status
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DNSRecordList is a list of DNSRecord resources.
type DNSRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// Items is the list of DNSRecords.
	Items []DNSRecord `json:"items"`
}

// DNSRecordSpec is the spec of a DNSRecord resource.
type DNSRecordSpec struct {
	// DefaultSpec is a structure containing common fields used by all extension resources.
	DefaultSpec `json:",inline"`
	// SecretRef is a reference to a secret that contains the cloud provider specific credentials.
	SecretRef corev1.SecretReference `json:"secretRef"`
	// Region is the region of this DNS record. If not specified, the region specified in SecretRef will be used.
	// If that is also not specified, the extension controller will use its default region.
	// +optional
	Region *string `json:"region,omitempty"`
	// Zone is the DNS hosted zone of this DNS record. If not specified, it will be determined automatically by
	// getting all hosted zones of the account and searching for the longest zone name that is a suffix of Name.
	// +optional
	Zone *string `json:"zone,omitempty"`
	// Name is the fully qualified domain name, e.g. "api.<shoot domain>". This field is immutable.
	Name string `json:"name"`
	// RecordType is the DNS record type. Only A, CNAME, and TXT records are currently supported. This field is immutable.
	RecordType DNSRecordType `json:"recordType"`
	// Values is a list of IP addresses for A records, a single hostname for CNAME records, or a list of texts for TXT records.
	Values []string `json:"values"`
	// TTL is the time to live in seconds. Defaults to 120.
	// +optional
	TTL *int64 `json:"ttl,omitempty"`
}

// DNSRecordStatus is the status of a DNSRecord resource.
type DNSRecordStatus struct {
	// DefaultStatus is a structure containing common fields used by all extension resources.
	DefaultStatus `json:",inline"`
	// Zone is the DNS hosted zone of this DNS record.
	// +optional
	Zone *string `json:"zone,omitempty"`
}

// DNSRecordType is a string alias.
type DNSRecordType string

const (
	// DNSRecordTypeA specifies that the DNSRecord is of type A.
	DNSRecordTypeA DNSRecordType = "A"
	// DNSRecordTypeAAAA specifies that the DNSRecord is of type AAAA.
	DNSRecordTypeAAAA DNSRecordType = "AAAA"
	// DNSRecordTypeCNAME specifies that the DNSRecord is of type CNAME.
	DNSRecordTypeCNAME DNSRecordType = "CNAME"
	// DNSRecordTypeTXT specifies that the DNSRecord is of type TXT.
	DNSRecordTypeTXT DNSRecordType = "TXT"
)

const (
	// ConditionTypeCreated specifies the condition type "Created" used as marker if record creation
	// on infrastructure was performed successfully at least once.
	ConditionTypeCreated = "Created"
)
