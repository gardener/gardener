// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AdminKubeconfigRequest can be used to request a kubeconfig with admin credentials
// for a Shoot cluster.
type AdminKubeconfigRequest struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec is the specification of the AdminKubeconfigRequest.
	Spec AdminKubeconfigRequestSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	// Status is the status of the AdminKubeconfigRequest.
	Status AdminKubeconfigRequestStatus `json:"status" protobuf:"bytes,3,opt,name=status"`
}

// AdminKubeconfigRequestStatus is the status of the AdminKubeconfigRequest containing
// the kubeconfig and expiration of the credential.
type AdminKubeconfigRequestStatus struct {
	// Kubeconfig contains the kubeconfig with cluster-admin privileges for the shoot cluster.
	Kubeconfig []byte `json:"kubeconfig" protobuf:"bytes,1,opt,name=kubeconfig"`
	// ExpirationTimestamp is the expiration timestamp of the returned credential.
	ExpirationTimestamp metav1.Time `json:"expirationTimestamp" protobuf:"bytes,2,opt,name=expirationTimestamp"`
}

// AdminKubeconfigRequestSpec contains the expiration time of the kubeconfig.
type AdminKubeconfigRequestSpec struct {
	// ExpirationSeconds is the requested validity duration of the credential. The
	// credential issuer may return a credential with a different validity duration so a
	// client needs to check the 'expirationTimestamp' field in a response.
	// Defaults to 1 hour.
	// +optional
	ExpirationSeconds *int64 `json:"expirationSeconds,omitempty" protobuf:"varint,1,opt,name=expirationSeconds"`
}
