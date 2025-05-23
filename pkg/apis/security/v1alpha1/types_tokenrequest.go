// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TokenRequest is a resource that is used to request WorkloadIdentity tokens.
type TokenRequest struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Spec holds configuration settings for the requested token.
	Spec TokenRequestSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	// Status bears the issued token with additional information back to the client.
	Status TokenRequestStatus `json:"status" protobuf:"bytes,3,opt,name=status"`
}

// TokenRequestSpec holds configuration settings for the requested token.
type TokenRequestSpec struct {
	// ContextObject identifies the object the token is requested for.
	// +optional
	ContextObject *ContextObject `json:"contextObject,omitempty" protobuf:"bytes,1,opt,name=contextObject"`
	// ExpirationSeconds specifies for how long the requested token should be valid.
	// +optional
	ExpirationSeconds *int64 `json:"expirationSeconds,omitempty" protobuf:"bytes,2,opt,name=expirationSeconds"`
}

// ContextObject identifies the object the token is requested for.
type ContextObject struct {
	// Kind of the object the token is requested for. Valid kinds are 'Shoot', 'Seed', etc.
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	// API version of the object the token is requested for.
	APIVersion string `json:"apiVersion" protobuf:"bytes,2,opt,name=apiVersion"`
	// Name of the object the token is requested for.
	Name string `json:"name" protobuf:"bytes,3,opt,name=name"`
	// Namespace of the object the token is requested for.
	// +optional
	Namespace *string `json:"namespace,omitempty" protobuf:"bytes,4,opt,name=namespace"`
	// UID of the object the token is requested for.
	UID types.UID `json:"uid" protobuf:"bytes,5,opt,name=uid,casttype=k8s.io/apimachinery/pkg/types.UID"`
}

// TokenRequestStatus bears the issued token with additional information back to the client.
type TokenRequestStatus struct {
	// Token is the issued token.
	Token string `json:"token" protobuf:"bytes,1,opt,name=token"`
	// ExpirationTimestamp is the time of expiration of the returned token.
	ExpirationTimestamp metav1.Time `json:"expirationTimestamp" protobuf:"bytes,2,opt,name=expirationTimestamp"`
}
