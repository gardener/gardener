// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:method=CreateToken,verb=create,subresource=token,input=github.com/gardener/gardener/pkg/apis/security/v1alpha1.TokenRequest,result=github.com/gardener/gardener/pkg/apis/security/v1alpha1.TokenRequest
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkloadIdentity is resource that allows workloads to be presented before external systems
// by giving them identities managed by the Gardener API server.
// The identity of such workload is represented by JSON Web Token issued by the Gardener API server.
// Workload identities are designed to be used by components running in the Gardener environment,
// seed or runtime cluster, that make use of identity federation inspired by the OIDC protocol.
type WorkloadIdentity struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Spec configures the JSON Web Token issued by the Gardener API server.
	Spec WorkloadIdentitySpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	// Status contain the latest observed status of the WorkloadIdentity.
	Status WorkloadIdentityStatus `json:"status" protobuf:"bytes,3,opt,name=status"`
}

// WorkloadIdentitySpec configures the JSON Web Token issued by the Gardener API server.
type WorkloadIdentitySpec struct {
	// Audiences specify the list of recipients that the JWT is intended for.
	// The values of this field will be set in the 'aud' claim.
	Audiences []string `json:"audiences" protobuf:"bytes,1,opt,name=audiences"`
	// TargetSystem represents specific configurations for the system that will accept the JWTs.
	TargetSystem TargetSystem `json:"targetSystem" protobuf:"bytes,2,opt,name=targetSystem"`
}

// TargetSystem represents specific configurations for the system that will accept the JWTs.
type TargetSystem struct {
	// Type is the type of the target system.
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// ProviderConfig is the configuration passed to extension resource.
	// +optional
	ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty" protobuf:"bytes,2,opt,name=providerConfig"`
}

// WorkloadIdentityStatus contain the latest observed status of the WorkloadIdentity.
type WorkloadIdentityStatus struct {
	// Sub contains the computed value of the subject that is going to be set in JWTs 'sub' claim.
	Sub string `json:"sub" protobuf:"bytes,1,opt,name=sub"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkloadIdentityList is a collection of WorkloadIdentities.
type WorkloadIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of WorkloadIdentities.
	Items []WorkloadIdentity `json:"items" protobuf:"bytes,2,rep,name=items"`
}
