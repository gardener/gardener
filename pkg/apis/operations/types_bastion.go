// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operations

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Bastion holds details about an SSH bastion for a shoot cluster.
type Bastion struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Specification of the Bastion.
	Spec BastionSpec
	// Most recently observed status of the Bastion.
	Status BastionStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BastionList is a list of Bastion objects.
type BastionList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of Bastion.
	Items []Bastion
}

// BastionSpec is the specification of a Bastion.
type BastionSpec struct {
	// ShootRef defines the target shoot for a Bastion. The name field of the ShootRef is immutable.
	ShootRef corev1.LocalObjectReference
	// SeedName is the name of the seed to which this Bastion is currently scheduled. This field is populated
	// at the beginning of a create/reconcile operation.
	SeedName *string
	// ProviderType is cloud provider used by the referenced Shoot.
	ProviderType *string
	// SSHPublicKey is the user's public key. This field is immutable.
	SSHPublicKey string
	// Ingress controls from where the created bastion host should be reachable.
	Ingress []BastionIngressPolicy
}

// BastionIngressPolicy represents an ingress policy for SSH bastion hosts.
type BastionIngressPolicy struct {
	// IPBlock defines an IP block that is allowed to access the bastion.
	IPBlock networkingv1.IPBlock
}

// BastionStatus holds the most recently observed status of the Bastion.
type BastionStatus struct {
	// Ingress holds the public IP and/or hostname of the bastion instance.
	Ingress *corev1.LoadBalancerIngress
	// Conditions represents the latest available observations of a Bastion's current state.
	Conditions []gardencore.Condition
	// LastHeartbeatTimestamp is the time when the bastion was last marked as
	// not to be deleted. When this is set, the ExpirationTimestamp is advanced
	// as well.
	LastHeartbeatTimestamp *metav1.Time
	// ExpirationTimestamp is the time after which a Bastion is supposed to be
	// garbage collected.
	ExpirationTimestamp *metav1.Time
	// ObservedGeneration is the most recent generation observed for this Bastion. It corresponds to the
	// Bastion's generation, which is updated on mutation by the API Server.
	ObservedGeneration *int64
}
