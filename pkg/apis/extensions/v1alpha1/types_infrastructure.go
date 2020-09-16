// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ Object = (*Infrastructure)(nil)

// InfrastructureResource is a constant for the name of the Infrastructure resource.
const InfrastructureResource = "Infrastructure"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Infrastructure is a specification for cloud provider infrastructure.
type Infrastructure struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructureSpec   `json:"spec"`
	Status InfrastructureStatus `json:"status"`
}

// GetExtensionSpec implements Object.
func (i *Infrastructure) GetExtensionSpec() Spec {
	return &i.Spec
}

// GetExtensionStatus implements Object.
func (i *Infrastructure) GetExtensionStatus() Status {
	return &i.Status
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureList is a list of Infrastructure resources.
type InfrastructureList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of Infrastructures.
	Items []Infrastructure `json:"items"`
}

// InfrastructureSpec is the spec for an Infrastructure resource.
type InfrastructureSpec struct {
	// DefaultSpec is a structure containing common fields used by all extension resources.
	DefaultSpec `json:",inline"`
	// Region is the region of this infrastructure.
	Region string `json:"region"`
	// SecretRef is a reference to a secret that contains the actual result of the generated cloud config.
	SecretRef corev1.SecretReference `json:"secretRef"`
	// SSHPublicKey is the public SSH key that should be used with this infrastructure.
	// +optional
	SSHPublicKey []byte `json:"sshPublicKey,omitempty"`
}

// InfrastructureStatus is the status for an Infrastructure resource.
type InfrastructureStatus struct {
	// DefaultStatus is a structure containing common fields used by all extension resources.
	DefaultStatus `json:",inline"`
	// NodesCIDR is the CIDR of the node network that was optionally created by the acting extension controller.
	// This might be needed in environments in which the CIDR for the network for the shoot worker node cannot
	// be statically defined in the Shoot resource but must be computed dynamically.
	// +optional
	NodesCIDR *string `json:"nodesCIDR,omitempty"`
}
