// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

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
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec contains the specification of the Backup Entry.
	Spec BackupEntrySpec
	// Status contains the most recently observed status of the Backup Entry.
	Status BackupEntryStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupEntryList is a list of BackupEntry objects.
type BackupEntryList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of BackupEntry.
	Items []BackupEntry
}

// BackupEntrySpec is the specification of a Backup Entry.
type BackupEntrySpec struct {
	// BucketName is the name of backup bucket for this Backup Entry.
	BucketName string
	// SeedName holds the name of the seed to which this BackupEntry is scheduled
	SeedName *string
}

// BackupEntryStatus holds the most recently observed status of the Backup Entry.
type BackupEntryStatus struct {
	// LastOperation holds information about the last operation on the BackupEntry.
	LastOperation *LastOperation
	// LastError holds information about the last occurred error during an operation.
	LastError *LastError
	// ObservedGeneration is the most recent generation observed for this BackupEntry. It corresponds to the
	// BackupEntry's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64
	// SeedName is the name of the seed to which this BackupEntry is currently scheduled. This field is populated
	// at the beginning of a create/reconcile operation. It is used when moving the BackupEntry between seeds.
	SeedName *string
	// MigrationStartTime is the time when a migration to a different seed was initiated.
	MigrationStartTime *metav1.Time
}
