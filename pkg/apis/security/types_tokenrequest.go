// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package security

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TokenRequest is a resource that is used to request WorkloadIdentity tokens.
type TokenRequest struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec holds configuration settings for the requested token.
	Spec TokenRequestSpec
	// Status bears the issued token with additional information back to the client.
	Status TokenRequestStatus
}

// TokenRequestSpec holds configuration settings for the requested token.
type TokenRequestSpec struct {
	// ContextObject identifies the object the token is requested for.
	ContextObject *ContextObject
	// ExpirationSeconds specifies for how long the requested token should be valid.
	ExpirationSeconds int64
}

// ContextObject identifies the object the token is requested for.
type ContextObject struct {
	// Kind of the object the token is requested for. Valid kinds are 'Shoot', 'Seed', etc.
	Kind string
	// API version of the object the token is requested for.
	APIVersion string
	// Name of the object the token is requested for.
	Name string
	// Namespace of the object the token is requested for.
	Namespace *string
	// UID of the object the token is requested for.
	UID types.UID
}

// TokenRequestStatus bears the issued token with additional information back to the client.
type TokenRequestStatus struct {
	// Token is the issued token.
	Token string
	// ExpirationTimestamp is the time of expiration of the returned token.
	ExpirationTimestamp metav1.Time
}
