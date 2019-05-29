// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

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
	metav1.ObjectMeta `json:"metadata"`
	// Spec contains the specification of the Backup Entry.
	// +optional
	Spec BackupEntrySpec `json:"spec,omitempty"`
	// Status contains the most recently observed status of the Backup Entry.
	// +optional
	Status BackupEntryStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupEntryList is a list of BackupEntry objects.
type BackupEntryList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of BackupEntry.
	Items []BackupEntry `json:"items"`
}

// BackupEntrySpec is the specification of a Backup Entry.
type BackupEntrySpec struct {
	// BucketName is the name of backup bucket for this Backup Entry.
	BucketName string `json:"bucketName"`
	// Seed holds the name of the seed allocated to BackupEntry for running controller.
	// +optional
	Seed *string `json:"seed,omitempty"`
}

// BackupEntryStatus holds the most recently observed status of the Backup Entry.
type BackupEntryStatus struct {
	// LastOperation holds information about the last operation on the BackupEntry.
	// +optional
	LastOperation *LastOperation `json:"lastOperation,omitempty"`
	// LastError holds information about the last occurred error during an operation.
	// +optional
	LastError *LastError `json:"lastError,omitempty"`
	// ObservedGeneration is the most recent generation observed for this BackupEntry. It corresponds to the
	// BackupEntry's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}
