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

package core

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerInstallation represents an installation request for an external controller.
type ControllerInstallation struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec contains the specification of this installation.
	Spec ControllerInstallationSpec
	// Status contains the status of this installation.
	Status ControllerInstallationStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerInstallationList is a collection of ControllerInstallations.
type ControllerInstallationList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of ControllerInstallations.
	Items []ControllerInstallation
}

// ControllerInstallationSpec is the specification of a ControllerInstallation.
type ControllerInstallationSpec struct {
	// RegistrationRef is used to reference a ControllerRegistration resources.
	RegistrationRef corev1.ObjectReference
	// SeedRef is used to reference a Seed resources.
	SeedRef corev1.ObjectReference
}

// ControllerInstallationStatus is the status of a ControllerInstallation.
type ControllerInstallationStatus struct {
	// Conditions represents the latest available observations of a ControllerInstallations's current state.
	Conditions []Condition
	// ProviderStatus contains type-specific status.
	// +optional
	ProviderStatus *ProviderConfig
}

const (
	// ControllerInstallationValid is a condition type for indicating whether the installation request is valid.
	ControllerInstallationValid ConditionType = "Valid"

	// ControllerInstallationInstalled is a condition type for indicating whether the controller has been installed.
	ControllerInstallationInstalled ConditionType = "Installed"
)
