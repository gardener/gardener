// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// BackupEntryForceDeletion is a constant for an annotation on a BackupEntry indicating that it should be force deleted.
	BackupEntryForceDeletion = "backupentry.core.gardener.cloud/force-deletion"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupEntry holds details about shoot backup.
type BackupEntry struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`

	// Spec contains the specification of the Backup Entry.
	// +optional
	Spec BackupEntrySpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// Status contains the most recently observed status of the Backup Entry.
	// +optional
	Status BackupEntryStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupEntryList is a list of BackupEntry objects.
type BackupEntryList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is the list of BackupEntry.
	Items []BackupEntry `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// BackupEntrySpec is the specification of a Backup Entry.
type BackupEntrySpec struct {
	// BucketName is the name of backup bucket for this Backup Entry.
	BucketName string `json:"bucketName" protobuf:"bytes,1,opt,name=bucketName"`
	// SeedName holds the name of the seed to which this BackupEntry is scheduled
	// +optional
	SeedName *string `json:"seedName,omitempty" protobuf:"bytes,2,opt,name=seedName"`
}

// BackupEntryStatus holds the most recently observed status of the Backup Entry.
type BackupEntryStatus struct {
	// LastOperation holds information about the last operation on the BackupEntry.
	// +optional
	LastOperation *LastOperation `json:"lastOperation,omitempty" protobuf:"bytes,1,opt,name=lastOperation"`
	// LastError holds information about the last occurred error during an operation.
	// +optional
	LastError *LastError `json:"lastError,omitempty" protobuf:"bytes,2,opt,name=lastError"`
	// ObservedGeneration is the most recent generation observed for this BackupEntry. It corresponds to the
	// BackupEntry's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,3,opt,name=observedGeneration"`
	// SeedName is the name of the seed to which this BackupEntry is currently scheduled. This field is populated
	// at the beginning of a create/reconcile operation. It is used when moving the BackupEntry between seeds.
	// +optional
	SeedName *string `json:"seedName,omitempty" protobuf:"bytes,4,opt,name=seedName"`
	// MigrationStartTime is the time when a migration to a different seed was initiated.
	// +optional
	MigrationStartTime *metav1.Time `json:"migrationStartTime,omitempty" protobuf:"bytes,5,opt,name=migrationStartTime"`
}
