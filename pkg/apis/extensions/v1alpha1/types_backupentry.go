// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ Object = (*BackupEntry)(nil)

// BackupEntryResource is a constant for the name of the BackupEntry resource.
const BackupEntryResource = "BackupEntry"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster,path=backupentries,shortName=be,singular=backupentry
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Type,JSONPath=".spec.type",type=string,description="The type of the cloud provider for this resource."
// +kubebuilder:printcolumn:name=Region,JSONPath=".spec.region",type=string,description="The region into which the backup entry should be created."
// +kubebuilder:printcolumn:name=Bucket,JSONPath=".spec.bucketName",type=string,description="The name of the bucket into which the backup entry should be created."
// +kubebuilder:printcolumn:name=State,JSONPath=".status.lastOperation.state",type=string,description="status of the last operation, one of Aborted, Processing, Succeeded, Error, Failed"
// +kubebuilder:printcolumn:name=Age,JSONPath=".metadata.creationTimestamp",type=date,description="creation timestamp"

// BackupEntry is a specification for backup Entry.
type BackupEntry struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the BackupEntry.
	// If the object's deletion timestamp is set, this field is immutable.
	Spec BackupEntrySpec `json:"spec"`
	// +optional
	Status BackupEntryStatus `json:"status"`
}

// GetExtensionSpec implements Object.
func (i *BackupEntry) GetExtensionSpec() Spec {
	return &i.Spec
}

// GetExtensionStatus implements Object.
func (i *BackupEntry) GetExtensionStatus() Status {
	return &i.Status
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupEntryList is a list of BackupEntry resources.
type BackupEntryList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of BackupEntry.
	Items []BackupEntry `json:"items"`
}

// BackupEntrySpec is the spec for an BackupEntry resource.
type BackupEntrySpec struct {
	// DefaultSpec is a structure containing common fields used by all extension resources.
	DefaultSpec `json:",inline"`
	// BackupBucketProviderStatus contains the provider status that has
	// been generated by the controller responsible for the `BackupBucket` resource.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	BackupBucketProviderStatus *runtime.RawExtension `json:"backupBucketProviderStatus,omitempty"`
	// Region is the region of this Entry. This field is immutable.
	Region string `json:"region"`
	// BucketName is the name of backup bucket for this Backup Entry.
	BucketName string `json:"bucketName"`
	// SecretRef is a reference to a secret that contains the credentials to access object store.
	SecretRef corev1.SecretReference `json:"secretRef"`
}

// BackupEntryStatus is the status for an BackupEntry resource.
type BackupEntryStatus struct {
	// DefaultStatus is a structure containing common fields used by all extension resources.
	DefaultStatus `json:",inline"`
}
