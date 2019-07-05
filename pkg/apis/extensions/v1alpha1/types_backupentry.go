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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ Object = (*BackupEntry)(nil)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupEntry is a specification for backup Entry.
type BackupEntry struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupEntrySpec   `json:"spec"`
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
	// Region is the region of this Entry.
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
