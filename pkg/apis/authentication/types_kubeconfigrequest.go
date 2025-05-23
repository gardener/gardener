// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubeconfigRequest can be used to request a kubeconfig for a Shoot cluster.
type KubeconfigRequest struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec is the specification of the KubeconfigRequest.
	Spec KubeconfigRequestSpec
	// Status is the status of the KubeconfigRequest.
	Status KubeconfigRequestStatus
}

// KubeconfigRequestSpec contains the expiration time of the kubeconfig.
type KubeconfigRequestSpec struct {
	// ExpirationSeconds is the requested validity duration of the credential. The credential issuer may return a
	// credential with a different validity duration so a client needs to check the 'expirationTimestamp' field in a
	// response.
	// Defaults to 1 hour.
	ExpirationSeconds int64
}

// KubeconfigRequestStatus is the status of the KubeconfigRequest containing the kubeconfig and expiration of the
// credential.
type KubeconfigRequestStatus struct {
	// Kubeconfig contains the kubeconfig for the shoot cluster.
	Kubeconfig []byte
	// ExpirationTimestamp is the expiration timestamp of the returned credential.
	ExpirationTimestamp metav1.Time
}
