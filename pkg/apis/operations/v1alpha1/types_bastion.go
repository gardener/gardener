// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

const (
	// BastionReady is a condition type for indicating whether the bastion has been
	// successfully reconciled on the seed cluster and is available to be used.
	BastionReady gardencorev1beta1.ConditionType = "BastionReady"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Bastion holds details about an SSH bastion for a shoot cluster.
type Bastion struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	// Specification of the Bastion.
	Spec BastionSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	// Most recently observed status of the Bastion.
	// +optional
	Status BastionStatus `json:"status" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BastionList is a list of Bastion objects.
type BastionList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of Bastion.
	Items []Bastion `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// BastionSpec is the specification of a Bastion.
type BastionSpec struct {
	// ShootRef defines the target shoot for a Bastion. The name field of the ShootRef is immutable.
	ShootRef corev1.LocalObjectReference `json:"shootRef" protobuf:"bytes,1,opt,name=shootRef"`
	// SeedName is the name of the seed to which this Bastion is currently scheduled. This field is populated
	// at the beginning of a create/reconcile operation.
	// +optional
	SeedName *string `json:"seedName,omitempty" protobuf:"bytes,2,opt,name=seedName"`
	// ProviderType is cloud provider used by the referenced Shoot.
	// +optional
	ProviderType *string `json:"providerType,omitempty" protobuf:"bytes,3,opt,name=providerType"`
	// SSHPublicKey is the user's public key. This field is immutable.
	SSHPublicKey string `json:"sshPublicKey" protobuf:"bytes,4,opt,name=sshPublicKey"`
	// Ingress controls from where the created bastion host should be reachable.
	Ingress []BastionIngressPolicy `json:"ingress" protobuf:"bytes,5,opt,name=ingress"`
}

// BastionIngressPolicy represents an ingress policy for SSH bastion hosts.
type BastionIngressPolicy struct {
	// IPBlock defines an IP block that is allowed to access the bastion.
	IPBlock networkingv1.IPBlock `json:"ipBlock" protobuf:"bytes,1,opt,name=ipBlock"`
}

// BastionStatus holds the most recently observed status of the Bastion.
type BastionStatus struct {
	// Ingress holds the public IP and/or hostname of the bastion instance.
	// +optional
	Ingress *corev1.LoadBalancerIngress `json:"ingress,omitempty" protobuf:"bytes,1,opt,name=ingress"`
	// Conditions represents the latest available observations of a Bastion's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []gardencorev1beta1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,2,rep,name=conditions"`
	// LastHeartbeatTimestamp is the time when the bastion was last marked as
	// not to be deleted. When this is set, the ExpirationTimestamp is advanced
	// as well.
	// +optional
	LastHeartbeatTimestamp *metav1.Time `json:"lastHeartbeatTimestamp,omitempty" protobuf:"bytes,3,opt,name=lastHeartbeatTimestamp"`
	// ExpirationTimestamp is the time after which a Bastion is supposed to be
	// garbage collected.
	// +optional
	ExpirationTimestamp *metav1.Time `json:"expirationTimestamp,omitempty" protobuf:"bytes,4,opt,name=expirationTimestamp"`
	// ObservedGeneration is the most recent generation observed for this Bastion. It corresponds to the
	// Bastion's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty" protobuf:"varint,5,opt,name=observedGeneration"`
}
