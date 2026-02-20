// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ Object = (*SelfHostedShootExposure)(nil)

// SelfHostedShootExposureResource is a constant for the name of the SelfHostedShootExposure resource.
const SelfHostedShootExposureResource = "SelfHostedShootExposure"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,path=selfhostedshootexposures,shortName=exp,singular=selfhostedshootexposure
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Type,JSONPath=".spec.type",type=string,description="The type of the self hosted shoot exposure provider for this resource."
// +kubebuilder:printcolumn:name=IP,JSONPath=".status.ingress[0].ip",type=string,description="The IP of the first LoadBalancer ingress."
// +kubebuilder:printcolumn:name=Hostname,JSONPath=".status.ingress[0].hostname",type=string,description="The Hostname of the first LoadBalancer ingress."
// +kubebuilder:printcolumn:name=Status,JSONPath=".status.lastOperation.state",type=string,description="Status of self hosted shoot exposure resource."
// +kubebuilder:printcolumn:name=Age,JSONPath=".metadata.creationTimestamp",type=date,description="creation timestamp"

// SelfHostedShootExposure contains the configuration for the exposure of a self-hosted shoot control plane.
type SelfHostedShootExposure struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the SelfHostedShootExposure.
	// If the object's deletion timestamp is set, this field is immutable.
	Spec SelfHostedShootExposureSpec `json:"spec"`
	// +optional
	Status SelfHostedShootExposureStatus `json:"status"`
}

// GetExtensionSpec implements Object.
func (s *SelfHostedShootExposure) GetExtensionSpec() Spec {
	return &s.Spec
}

// GetExtensionStatus implements Object.
func (s *SelfHostedShootExposure) GetExtensionStatus() Status {
	return &s.Status
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SelfHostedShootExposureList is a list of SelfHostedShootExposure resources.
type SelfHostedShootExposureList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is the list of SelfHostedShootExposures.
	Items []SelfHostedShootExposure `json:"items"`
}

// SelfHostedShootExposureSpec is the spec for an SelfHostedShootExposure resource.
type SelfHostedShootExposureSpec struct {
	// DefaultSpec is a structure containing common fields used by all extension resources.
	DefaultSpec `json:",inline"`

	// CredentialsRef is a reference to the cloud provider credentials.
	// +optional
	CredentialsRef *corev1.ObjectReference `json:"credentialsRef,omitempty"`
	// Endpoints contains a list of healthy control plane nodes to expose.
	Endpoints []ControlPlaneEndpoint `json:"endpoints"`
}

// ControlPlaneEndpoint is an endpoint that should be exposed.
type ControlPlaneEndpoint struct {
	// NodeName is the name of the node to expose.
	NodeName string `json:"nodeName"`
	// Addresses is a list of addresses of type NodeAddress to expose.
	Addresses []corev1.NodeAddress `json:"addresses"`
	// Port of the API server.
	Port int32 `json:"port"`
}

// SelfHostedShootExposureStatus is the status for an SelfHostedShootExposure resource.
type SelfHostedShootExposureStatus struct {
	// DefaultStatus is a structure containing common fields used by all extension resources.
	DefaultStatus `json:",inline"`

	// Ingress is a list of endpoints of the exposure mechanism.
	// +optional
	Ingress []corev1.LoadBalancerIngress `json:"ingress,omitempty"`
}
