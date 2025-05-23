// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedmanagement

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagedSeedSet represents a set of identical ManagedSeeds.
type ManagedSeedSet struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec defines the desired identities of ManagedSeeds and Shoots in this set.
	Spec ManagedSeedSetSpec
	// Status is the current status of ManagedSeeds and Shoots in this ManagedSeedSet.
	Status ManagedSeedSetStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagedSeedSetList is a list of ManagedSeed objects.
type ManagedSeedSetList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of ManagedSeedSets.
	Items []ManagedSeedSet
}

// ManagedSeedSetSpec is the specification of a ManagedSeedSet.
type ManagedSeedSetSpec struct {
	// Replicas is the desired number of replicas of the given Template. Defaults to 1.
	Replicas *int32
	// Selector is a label query over ManagedSeeds and Shoots that should match the replica count.
	// It must match the ManagedSeeds and Shoots template's labels. This field is immutable.
	Selector metav1.LabelSelector
	// Template describes the ManagedSeed that will be created if insufficient replicas are detected.
	// Each ManagedSeed created / updated by the ManagedSeedSet will fulfill this template.
	Template ManagedSeedTemplate
	// ShootTemplate describes the Shoot that will be created if insufficient replicas are detected for hosting the corresponding ManagedSeed.
	// Each Shoot created / updated by the ManagedSeedSet will fulfill this template.
	ShootTemplate gardencore.ShootTemplate
	// UpdateStrategy specifies the UpdateStrategy that will be
	// employed to update ManagedSeeds / Shoots in the ManagedSeedSet when a revision is made to
	// Template / ShootTemplate.
	UpdateStrategy *UpdateStrategy
	// RevisionHistoryLimit is the maximum number of revisions that will be maintained
	// in the ManagedSeedSet's revision history. Defaults to 10. This field is immutable.
	RevisionHistoryLimit *int32
}

// UpdateStrategy specifies the strategy that the ManagedSeedSet
// controller will use to perform updates. It includes any additional parameters
// necessary to perform the update for the indicated strategy.
type UpdateStrategy struct {
	// Type indicates the type of the UpdateStrategy. Defaults to RollingUpdate.
	Type *UpdateStrategyType
	// RollingUpdate is used to communicate parameters when Type is RollingUpdateStrategyType.
	RollingUpdate *RollingUpdateStrategy
}

// UpdateStrategyType is a string enumeration type that enumerates
// all possible update strategies for the ManagedSeedSet controller.
type UpdateStrategyType string

const (
	// RollingUpdateStrategyType indicates that update will be
	// applied to all ManagedSeeds / Shoots in the ManagedSeedSet with respect to the ManagedSeedSet
	// ordering constraints.
	RollingUpdateStrategyType UpdateStrategyType = "RollingUpdate"
)

// RollingUpdateStrategy is used to communicate parameter for RollingUpdateStrategyType.
type RollingUpdateStrategy struct {
	// Partition indicates the ordinal at which the ManagedSeedSet should be partitioned. Defaults to 0.
	Partition *int32
}

// ManagedSeedSetStatus represents the current state of a ManagedSeedSet.
type ManagedSeedSetStatus struct {
	// ObservedGeneration is the most recent generation observed for this ManagedSeedSet. It corresponds to the
	// ManagedSeedSet's generation, which is updated on mutation by the API Server.
	ObservedGeneration int64
	// Replicas is the number of replicas (ManagedSeeds and their corresponding Shoots) created by the ManagedSeedSet controller.
	Replicas int32
	// ReadyReplicas is the number of ManagedSeeds created by the ManagedSeedSet controller that have a Ready Condition.
	ReadyReplicas int32
	// NextReplicaNumber is the ordinal number that will be assigned to the next replica of the ManagedSeedSet.
	NextReplicaNumber int32
	// CurrentReplicas is the number of ManagedSeeds created by the ManagedSeedSet controller from the ManagedSeedSet version
	// indicated by CurrentRevision.
	CurrentReplicas int32
	// UpdatedReplicas is the number of ManagedSeeds created by the ManagedSeedSet controller from the ManagedSeedSet version
	// indicated by UpdateRevision.
	UpdatedReplicas int32
	// CurrentRevision, if not empty, indicates the version of the ManagedSeedSet used to generate ManagedSeeds with smaller
	// ordinal numbers during updates.
	CurrentRevision string
	// UpdateRevision, if not empty, indicates the version of the ManagedSeedSet used to generate ManagedSeeds with larger
	// ordinal numbers during updates
	UpdateRevision string
	// CollisionCount is the count of hash collisions for the ManagedSeedSet. The ManagedSeedSet controller
	// uses this field as a collision avoidance mechanism when it needs to create the name for the
	// newest ControllerRevision.
	CollisionCount *int32
	// Conditions represents the latest available observations of a ManagedSeedSet's current state.
	Conditions []gardencore.Condition
	// PendingReplica, if not empty, indicates the replica that is currently pending creation, update, or deletion.
	// This replica is in a state that requires the controller to wait for it to change before advancing to the next replica.
	PendingReplica *PendingReplica
}

// PendingReplicaReason is a string enumeration type that enumerates all possible reasons for a replica to be pending.
type PendingReplicaReason string

const (
	// ShootReconcilingReason indicates that the replica's shoot is reconciling.
	ShootReconcilingReason PendingReplicaReason = "ShootReconciling"
	// ShootDeletingReason indicates that the replica's shoot is deleting.
	ShootDeletingReason PendingReplicaReason = "ShootDeleting"
	// ShootReconcileFailedReason indicates that the reconciliation of this replica's shoot has failed.
	ShootReconcileFailedReason PendingReplicaReason = "ShootReconcileFailed"
	// ShootDeleteFailedReason indicates that the deletion of tis replica's shoot has failed.
	ShootDeleteFailedReason PendingReplicaReason = "ShootDeleteFailed"
	// ManagedSeedPreparingReason indicates that the replica's managed seed is preparing.
	ManagedSeedPreparingReason PendingReplicaReason = "ManagedSeedPreparing"
	// ManagedSeedDeletingReason indicates that the replica's managed seed is deleting.
	ManagedSeedDeletingReason PendingReplicaReason = "ManagedSeedDeleting"
	// SeedNotReadyReason indicates that the replica's seed is not ready.
	SeedNotReadyReason PendingReplicaReason = "SeedNotReady"
	// ShootNotHealthyReason indicates that the replica's shoot is not healthy.
	ShootNotHealthyReason PendingReplicaReason = "ShootNotHealthy"
)

// PendingReplica contains information about a replica that is currently pending creation, update, or deletion.
type PendingReplica struct {
	// Name is the replica name.
	Name string
	// Reason is the reason for the replica to be pending.
	Reason PendingReplicaReason
	// Since is the moment in time since the replica is pending with the specified reason.
	Since metav1.Time
	// Retries is the number of times the shoot operation (reconcile or delete) has been retried after having failed.
	// Only applicable if Reason is ShootReconciling or ShootDeleting.
	Retries *int32
}

// TODO Condition constants
