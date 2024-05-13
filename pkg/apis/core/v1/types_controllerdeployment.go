// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerDeployment contains information about how this controller is deployed.
type ControllerDeployment struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Helm configures that an extension controller is deployed using helm.
	// +optional
	Helm *HelmControllerDeployment `json:"helm,omitempty" protobuf:"bytes,2,opt,name=helm"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerDeploymentList is a collection of ControllerDeployments.
type ControllerDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of ControllerDeployments.
	Items []ControllerDeployment `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// HelmControllerDeployment configures how an extension controller is deployed using helm.
type HelmControllerDeployment struct {
	// RawChart is the base64-encoded, gzip'ed, tar'ed extension controller chart.
	// +optional
	RawChart []byte `json:"rawChart,omitempty" protobuf:"bytes,1,opt,name=rawChart"`
	// Values are the chart values.
	// +optional
	Values *apiextensionsv1.JSON `json:"values,omitempty" protobuf:"bytes,2,opt,name=values"`
}
