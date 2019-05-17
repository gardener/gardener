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
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupInfrastructure is a specification for cloud provider backup infrastructure.
type BackupInfrastructure struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupInfrastructureSpec   `json:"spec"`
	Status BackupInfrastructureStatus `json:"status"`
}

// GetExtensionType returns the type of this BackupInfrastructure resource.
func (i *BackupInfrastructure) GetExtensionType() string {
	return i.Spec.Type
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupInfrastructureList is a list of BackupInfrastructure resources.
type BackupInfrastructureList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of BackupInfrastructures.
	Items []BackupInfrastructure `json:"items"`
}

// BackupInfrastructureSpec is the spec for a BackupInfrastructure resource.
type BackupInfrastructureSpec struct {
	// DefaultSpec is a structure containing common fields used by all extension resources.
	DefaultSpec `json:",inline"`

	// ProviderConfig contains provider-specific configuration for this backup infrastructure.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty"`
	// Region is the region of this backup infrastructure.
	Region string `json:"region"`
	// SecretRef is a reference to a secret that contains credentials needed for creating the backup infrastructure.
	SecretRef corev1.SecretReference `json:"secretRef"`
}

// BackupInfrastructureStatus is the status for a BackupInfrastructure resource.
type BackupInfrastructureStatus struct {
	// DefaultStatus is a structure containing common fields used by all extension resources.
	DefaultStatus `json:",inline"`

	// ProviderStatus contains provider-specific output for this backup infrastructure.
	// +optional
	ProviderStatus *runtime.RawExtension `json:"providerStatus,omitempty"`
}
