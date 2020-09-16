// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	ProviderStatus *runtime.RawExtension
}

const (
	// ControllerInstallationHealthy is a condition type for indicating whether the controller is healthy.
	ControllerInstallationHealthy ConditionType = "Healthy"
	// ControllerInstallationInstalled is a condition type for indicating whether the controller has been installed.
	ControllerInstallationInstalled ConditionType = "Installed"
	// ControllerInstallationValid is a condition type for indicating whether the installation request is valid.
	ControllerInstallationValid ConditionType = "Valid"
	// ControllerInstallationRequired is a condition type for indicating that the respective extension controller is
	// still required on the seed cluster as corresponding extension resources still exist.
	ControllerInstallationRequired ConditionType = "Required"
)
