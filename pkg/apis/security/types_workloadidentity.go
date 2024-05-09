// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package security

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkloadIdentity is resource that allows Gardener API server to issue JSON Web Tokens.
// It is designed to be used by components running in the Gardener environment, seed or runtime cluster,
// that make use of identity federation based on OIDC protocol.
type WorkloadIdentity struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec configures the JSON Web Token issued by the Gardener API server.
	Spec WorkloadIdentitySpec
	// Status contain the latest observed status of the WorkloadIdentity.
	Status WorkloadIdentityStatus
}

// WorkloadIdentitySpec configures the JSON Web Token issued by the Gardener API server.
type WorkloadIdentitySpec struct {
	// Audiences specify the list of OIDC audiences that will be set in the 'aud' claim.
	Audiences []string
	// TargetSystem represents specific configurations for the system that will accept the JWTs.
	TargetSystem TargetSystem
}

// TargetSystem represents specific configurations for the system that will accept the JWTs.
type TargetSystem struct {
	// Type is the type of the target system.
	Type string
	// ProviderConfig is the configuration passed to extension resource.
	ProviderConfig *runtime.RawExtension
}

// WorkloadIdentityStatus contain the latest observed status of the WorkloadIdentity.
type WorkloadIdentityStatus struct {
	// Sub contains the computed value for the 'sub' OIDC claim.
	Sub string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkloadIdentityList is a collection of WorkloadIdentities.
type WorkloadIdentityList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of WorkloadIdentities.
	Items []WorkloadIdentity
}
