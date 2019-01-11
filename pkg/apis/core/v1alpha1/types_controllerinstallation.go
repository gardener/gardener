// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerInstallation represents an installation request for an external controller.
type ControllerInstallation struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec contains the specification of this installation.
	Spec ControllerInstallationSpec `json:"spec,omitempty"`
	// Status contains the status of this installation.
	Status ControllerInstallationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerInstallationList is a collection of ControllerInstallations.
type ControllerInstallationList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of ControllerInstallations.
	Items []ControllerInstallation `json:"items"`
}

// ControllerInstallationSpec is the specification of a ControllerInstallation.
type ControllerInstallationSpec struct {
	// RegistrationRef is used to reference a ControllerRegistration resources.
	RegistrationRef corev1.ObjectReference `json:"registrationRef"`
	// SeedRef is used to reference a Seed resources.
	SeedRef corev1.ObjectReference `json:"seedRef"`
}

// ControllerInstallationStatus is the status of a ControllerInstallation.
type ControllerInstallationStatus struct {
	// Conditions represents the latest available observations of a ControllerInstallations's current state.
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
	// ProviderStatus contains type-specific status.
	// +optional
	ProviderStatus *ProviderConfig `json:"providerStatus,omitempty"`
}

const (
	// ControllerInstallationValid is a condition type for indicating whether the installation request is valid.
	ControllerInstallationValid ConditionType = "Valid"

	// ControllerInstallationInstalled is a condition type for indicating whether the controller has been installed.
	ControllerInstallationInstalled ConditionType = "Installed"
)
