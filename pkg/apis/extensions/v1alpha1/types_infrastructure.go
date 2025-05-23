// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
// +kubebuilder:resource:scope=Namespaced,path=infrastructures,shortName=infra,singular=infrastructure
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Type,JSONPath=".spec.type",type=string,description="The type of the cloud provider for this resource."
// +kubebuilder:printcolumn:name=Region,JSONPath=".spec.region",type=string,description="The region into which the infrastructure should be deployed."
// +kubebuilder:printcolumn:name=Status,JSONPath=".status.lastOperation.state",type=string,description="Status of infrastructure resource."
// +kubebuilder:printcolumn:name=Age,JSONPath=".metadata.creationTimestamp",type=date,description="creation timestamp"

// Infrastructure is a specification for cloud provider infrastructure.
type Infrastructure struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the Infrastructure.
	// If the object's deletion timestamp is set, this field is immutable.
	Spec InfrastructureSpec `json:"spec"`
	// +optional
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
	// Region is the region of this infrastructure. This field is immutable.
	Region string `json:"region"`
	// SecretRef is a reference to a secret that contains the cloud provider credentials.
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
	// EgressCIDRs is a list of CIDRs used by the shoot as the source IP for egress traffic. For certain environments the egress
	// IPs may not be stable in which case the extension controller may opt to not populate this field.
	// +optional
	EgressCIDRs []string `json:"egressCIDRs,omitempty"`
	// Networking contains information about cluster networking such as CIDRs.
	// +optional
	Networking *InfrastructureStatusNetworking `json:"networking,omitempty"`
}

// InfrastructureStatusNetworking is a structure containing information about the node, service and pod network ranges.
type InfrastructureStatusNetworking struct {
	// Pods are the CIDRs of the pod network.
	// +optional
	Pods []string `json:"pods,omitempty"`
	// Nodes are the CIDRs of the node network.
	// +optional
	Nodes []string `json:"nodes,omitempty"`
	// Services are the CIDRs of the service network.
	// +optional
	Services []string `json:"services,omitempty"`
}
