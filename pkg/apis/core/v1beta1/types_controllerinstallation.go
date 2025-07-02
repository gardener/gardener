// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

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
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec contains the specification of this installation.
	// If the object's deletion timestamp is set, this field is immutable.
	Spec ControllerInstallationSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// Status contains the status of this installation.
	Status ControllerInstallationStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerInstallationList is a collection of ControllerInstallations.
type ControllerInstallationList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Items is the list of ControllerInstallations.
	Items []ControllerInstallation `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ControllerInstallationSpec is the specification of a ControllerInstallation.
type ControllerInstallationSpec struct {
	// RegistrationRef is used to reference a ControllerRegistration resource.
	// The name field of the RegistrationRef is immutable.
	RegistrationRef corev1.ObjectReference `json:"registrationRef" protobuf:"bytes,1,opt,name=registrationRef"`
	// SeedRef is used to reference a Seed resource. The name field of the SeedRef is immutable.
	SeedRef corev1.ObjectReference `json:"seedRef" protobuf:"bytes,2,opt,name=seedRef"`
	// DeploymentRef is used to reference a ControllerDeployment resource.
	// +optional
	DeploymentRef *corev1.ObjectReference `json:"deploymentRef,omitempty" protobuf:"bytes,3,opt,name=deploymentRef"`
}

// ControllerInstallationStatus is the status of a ControllerInstallation.
type ControllerInstallationStatus struct {
	// Conditions represents the latest available observations of a ControllerInstallations's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// ProviderStatus contains type-specific status.
	// +optional
	ProviderStatus *runtime.RawExtension `json:"providerStatus,omitempty" protobuf:"bytes,2,opt,name=providerStatus"`
}

const (
	// ControllerInstallationHealthy is a condition type for indicating whether the controller is healthy.
	ControllerInstallationHealthy ConditionType = "Healthy"
	// ControllerInstallationInstalled is a condition type for indicating whether the controller has been installed.
	ControllerInstallationInstalled ConditionType = "Installed"
	// ControllerInstallationProgressing is a condition type for indicating whether the controller is progressing.
	ControllerInstallationProgressing ConditionType = "Progressing"
	// ControllerInstallationValid is a condition type for indicating whether the installation request is valid.
	ControllerInstallationValid ConditionType = "Valid"
	// ControllerInstallationRequired is a condition type for indicating that the respective extension controller is
	// still required on the seed cluster as corresponding extension resources still exist.
	ControllerInstallationRequired ConditionType = "Required"
)
