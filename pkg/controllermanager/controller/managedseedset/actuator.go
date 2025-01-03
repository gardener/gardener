// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseedset

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Actuator acts upon ManagedSeedSet resources.
type Actuator interface {
	// Reconcile reconciles ManagedSeedSet creation, update, or deletion.
	Reconcile(context.Context, logr.Logger, *seedmanagementv1alpha1.ManagedSeedSet) (*seedmanagementv1alpha1.ManagedSeedSetStatus, bool, error)
}

// actuator is a concrete implementation of Actuator.
type actuator struct {
	gardenClient   client.Client
	replicaGetter  ReplicaGetter
	replicaFactory ReplicaFactory
	cfg            *controllermanagerconfigv1alpha1.ManagedSeedSetControllerConfiguration
	recorder       record.EventRecorder
}

// NewActuator creates and returns a new Actuator with the given parameters.
func NewActuator(
	gardenClient client.Client,
	replicaGetter ReplicaGetter,
	replicaFactory ReplicaFactory,
	cfg *controllermanagerconfigv1alpha1.ManagedSeedSetControllerConfiguration,
	recorder record.EventRecorder,
) Actuator {
	return &actuator{
		gardenClient:   gardenClient,
		replicaFactory: replicaFactory,
		replicaGetter:  replicaGetter,
		cfg:            cfg,
		recorder:       recorder,
	}
}

// Now returns the current local time. Exposed for testing.
var Now = metav1.Now

// Reconcile reconciles ManagedSeedSet creation or update.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet) (status *seedmanagementv1alpha1.ManagedSeedSetStatus, removeFinalizer bool, err error) {
	// Initialize status
	status = managedSeedSet.Status.DeepCopy()
	status.ObservedGeneration = managedSeedSet.Generation

	defer func() {
		if err != nil {
			a.errorEventf(managedSeedSet, gardencorev1beta1.EventReconcileError, err.Error())
		}
	}()

	// Get replicas
	replicas, err := a.replicaGetter.GetReplicas(ctx, managedSeedSet)
	if err != nil {
		return status, false, err
	}

	// Sort replicas by ascending ordinal
	sort.Sort(ascendingOrdinal(replicas))

	// Get the pending replica, if any
	pendingReplica := getPendingReplica(replicas, status)

	// Determine ready, postponed, and deletable replicas
	var readyReplicas, postponedReplicas, deletableReplicas []Replica
	for _, r := range replicas {
		if replicaIsReady(r) {
			readyReplicas = append(readyReplicas, r)
		} else if r != pendingReplica {
			postponedReplicas = append(postponedReplicas, r)
		}
		if r.IsDeletable() {
			deletableReplicas = append(deletableReplicas, r)
		}
		debugReplica(r, log.V(1))
	}
	log.V(1).Info("Current replicas of ManagedSeedSet", "readyReplicas", readyReplicas, "postponedReplicas", postponedReplicas, "deletableReplicas", deletableReplicas)

	// Update replicas and readyReplicas in status
	status.Replicas = int32(len(replicas))           // #nosec G115 -- `ra.replicaGetter.GetReplicas(ctx, managedSeedSet)` returns a line for every ManagedSeeds in the system. This number cannot exceed max int32.
	status.ReadyReplicas = int32(len(readyReplicas)) // #nosec G115 -- `ra.replicaGetter.GetReplicas(ctx, managedSeedSet)` returns a line for every ManagedSeeds in the system. This number cannot exceed max int32.

	// Determine the actual and target replica counts
	count := len(replicas)
	targetCount := 0
	if managedSeedSet.DeletionTimestamp == nil {
		targetCount = int(*managedSeedSet.Spec.Replicas)
	}

	// Determine whether scaling out or in
	scalingOut, scalingIn := count < targetCount, count > targetCount

	// Reconcile the pending replica, if any
	if pendingReplica != nil {
		if pending, err := a.reconcileReplica(ctx, log, managedSeedSet, status, pendingReplica, scalingIn); err != nil || pending {
			return status, false, err
		}
	}

	switch {
	case scalingOut:
		// Initialize a new replica and create its shoot
		ordinal := getNextOrdinal(replicas, status)
		if err := a.createReplica(ctx, log, managedSeedSet, status, ordinal); err != nil {
			return status, false, err
		}

		// Increment Replicas and NextReplicaNumber in status
		status.Replicas++
		status.NextReplicaNumber = ordinal + 1

		return status, false, nil

	case scalingIn:
		// Determine the replica to be deleted
		// From all deletable replicas, choose the one with lowest priority
		if len(deletableReplicas) == 0 {
			return status, false, fmt.Errorf("no deletable replicas found")
		}
		sort.Sort(ascendingPriority(deletableReplicas))
		r := deletableReplicas[0]

		// Delete the replica's managed seed (if it exists), or its shoot (if not)
		if err := a.deleteReplica(ctx, log, managedSeedSet, status, r); err != nil {
			return status, false, err
		}

		// Decrement ReadyReplicas in status
		if replicaIsReady(r) {
			status.ReadyReplicas--
		}

		return status, false, nil
	}

	// Reconcile postponed replicas
	for _, r := range postponedReplicas {
		if pending, err := a.reconcileReplica(ctx, log, managedSeedSet, status, r, scalingIn); err != nil || pending {
			return status, false, err
		}
	}

	log.V(1).Info("Nothing to do")
	status.PendingReplica = nil
	return status, true, nil
}

// Event reason constants.
const (
	EventCreatingShoot                   = "CreatingShoot"
	EventDeletingShoot                   = "DeletingShoot"
	EventRetryingShootReconciliation     = "RetryingShootReconciliation"
	EventNotRetryingShootReconciliation  = "NotRetryingShootReconciliation"
	EventRetryingShootDeletion           = "RetryingShootDeletion"
	EventNotRetryingShootDeletion        = "NotRetryingShootDeletion"
	EventWaitingForShootReconciled       = "WaitingForShootReconciled"
	EventWaitingForShootDeleted          = "WaitingForShootDeleted"
	EventWaitingForShootHealthy          = "WaitingForShootHealthy"
	EventCreatingManagedSeed             = "CreatingManagedSeed"
	EventDeletingManagedSeed             = "DeletingManagedSeed"
	EventWaitingForManagedSeedRegistered = "WaitingForManagedSeedRegistered"
	EventWaitingForManagedSeedDeleted    = "WaitingForManagedSeedDeleted"
	EventWaitingForSeedReady             = "WaitingForSeedReady"
)

func (a *actuator) reconcileReplica(
	ctx context.Context,
	log logr.Logger,
	managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet,
	status *seedmanagementv1alpha1.ManagedSeedSetStatus,
	r Replica,
	scalingIn bool,
) (bool, error) {
	replicaStatus := r.GetStatus()
	log = log.WithValues("replica", r.GetObjectKey())

	switch {
	case replicaStatus == StatusShootReconcileFailed && !scalingIn:
		// This replica's shoot reconciliation has failed, retry it if max retries is not yet reached
		retries := getPendingReplicaRetries(status, r.GetName(), seedmanagementv1alpha1.ShootReconcilingReason)
		if int(retries) < *a.cfg.MaxShootRetries {
			log.Info("Retrying Shoot reconciliation")
			a.infoEventf(managedSeedSet, EventRetryingShootReconciliation, "Retrying Shoot %s reconciliation", r.GetFullName())
			if err := r.RetryShoot(ctx, a.gardenClient); err != nil {
				return false, err
			}
			updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootReconcilingReason, ptr.To(retries+1))
		} else {
			log.Info("Not retrying Shoot reconciliation since max retries have been reached", "maxRetries", *a.cfg.MaxShootRetries)
			a.infoEventf(managedSeedSet, EventNotRetryingShootReconciliation, "Not retrying Shoot %s reconciliation since max retries have been reached", r.GetFullName())
			updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootReconcileFailedReason, &retries)
		}
		return true, nil

	case replicaStatus == StatusShootDeleteFailed:
		// This replica's shoot deletion has failed, retry it if max retries is not yet reached
		retries := getPendingReplicaRetries(status, r.GetName(), seedmanagementv1alpha1.ShootDeletingReason)
		if int(retries) < *a.cfg.MaxShootRetries {
			log.Info("Retrying Shoot deletion")
			a.infoEventf(managedSeedSet, EventRetryingShootDeletion, "Retrying Shoot %s deletion", r.GetFullName())
			if err := r.RetryShoot(ctx, a.gardenClient); err != nil {
				return false, err
			}
			updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootDeletingReason, ptr.To(retries+1))
		} else {
			log.Info("Not retrying Shoot deletion since max retries have been reached", "maxRetries", *a.cfg.MaxShootRetries)
			a.infoEventf(managedSeedSet, EventNotRetryingShootDeletion, "Not retrying Shoot %s deletion since max retries have been reached", r.GetFullName())
			updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootDeleteFailedReason, &retries)
		}
		return true, nil

	case replicaStatus == StatusShootReconciling && !scalingIn:
		// This replica's shoot is reconciling, wait for it to be reconciled before moving to the next replica
		log.Info("Waiting for Shoot to be reconciled")
		a.infoEventf(managedSeedSet, EventWaitingForShootReconciled, "Waiting for Shoot %s to be reconciled", r.GetFullName())
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootReconcilingReason, nil)
		return true, nil

	case replicaStatus == StatusShootDeleting:
		// This replica's shoot is deleting, wait for it to be deleted before moving to the next replica
		log.Info("Waiting for Shoot to be deleted")
		a.infoEventf(managedSeedSet, EventWaitingForShootDeleted, "Waiting for Shoot %s to be deleted", r.GetFullName())
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootDeletingReason, nil)
		return true, nil

	case replicaStatus == StatusShootReconciled:
		// This replica's shoot is fully reconciled and its managed seed doesn't exist
		// If not scaling in, create its managed seed, otherwise delete its shoot
		if !scalingIn {
			log.Info("Creating ManagedSeed")
			a.infoEventf(managedSeedSet, EventCreatingManagedSeed, "Creating ManagedSeed %s", r.GetFullName())
			if err := r.CreateManagedSeed(ctx, a.gardenClient); err != nil {
				return false, err
			}
			updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil)
		} else {
			log.Info("Deleting Shoot")
			a.infoEventf(managedSeedSet, EventDeletingShoot, "Deleting Shoot %s", r.GetFullName())
			if err := r.DeleteShoot(ctx, a.gardenClient); err != nil {
				return false, err
			}
			updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootDeletingReason, nil)
		}
		return true, nil

	case replicaStatus == StatusManagedSeedPreparing && !scalingIn:
		// This replica's managed seed is preparing, wait for the it to be registered before moving to the next replica
		log.Info("Waiting for ManagedSeed to be registered")
		a.infoEventf(managedSeedSet, EventWaitingForManagedSeedRegistered, "Waiting for ManagedSeed %s to be registered", r.GetFullName())
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil)
		return true, nil

	case replicaStatus == StatusManagedSeedDeleting:
		// This replica's managed seed is deleting, wait for it to be deleted before moving to the next replica
		log.Info("Waiting for ManagedSeed to be deleted")
		a.infoEventf(managedSeedSet, EventWaitingForManagedSeedDeleted, "Waiting for ManagedSeed %s to be deleted", r.GetFullName())
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ManagedSeedDeletingReason, nil)
		return true, nil

	case !r.IsSeedReady() && !scalingIn:
		// This replica's seed is not ready, wait for it to be ready before moving to the next replica
		log.Info("Waiting for Seed to be ready")
		a.infoEventf(managedSeedSet, EventWaitingForSeedReady, "Waiting for Seed %s to be ready", r.GetName())
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.SeedNotReadyReason, nil)
		return true, nil

	case r.GetShootHealthStatus() != gardenerutils.ShootStatusHealthy && !scalingIn:
		// This replica's shoot is not healthy, wait for it to be healthy before moving to the next replica
		log.Info("Waiting for Shoot to be healthy")
		a.infoEventf(managedSeedSet, EventWaitingForShootHealthy, "Waiting for Shoot %s to be healthy", r.GetFullName())
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootNotHealthyReason, nil)
		return true, nil
	}

	return false, nil
}

func (a *actuator) createReplica(
	ctx context.Context,
	log logr.Logger,
	managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet,
	status *seedmanagementv1alpha1.ManagedSeedSetStatus,
	ordinal int32,
) error {
	r := a.replicaFactory.NewReplica(managedSeedSet, nil, nil, nil, false)

	fullName := getFullName(managedSeedSet, ordinal)
	log.Info("Creating Shoot", "replica", client.ObjectKey{Namespace: managedSeedSet.Namespace, Name: fullName})
	a.infoEventf(managedSeedSet, EventCreatingShoot, "Creating Shoot %s", fullName)
	if err := r.CreateShoot(ctx, a.gardenClient, ordinal); err != nil {
		return err
	}
	updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootReconcilingReason, nil)
	return nil
}

func (a *actuator) deleteReplica(
	ctx context.Context,
	log logr.Logger,
	managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet,
	status *seedmanagementv1alpha1.ManagedSeedSetStatus,
	r Replica,
) error {
	log = log.WithValues("replica", r.GetObjectKey())

	if replicaManagedSeedExists(r.GetStatus()) {
		log.Info("Deleting ManagedSeed")
		a.infoEventf(managedSeedSet, EventDeletingManagedSeed, "Deleting ManagedSeed %s", r.GetFullName())
		if err := r.DeleteManagedSeed(ctx, a.gardenClient); err != nil {
			return err
		}
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ManagedSeedDeletingReason, nil)
	} else {
		log.Info("Deleting Shoot")
		a.infoEventf(managedSeedSet, EventDeletingShoot, "Deleting Shoot %s", r.GetFullName())
		if err := r.DeleteShoot(ctx, a.gardenClient); err != nil {
			return err
		}
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootDeletingReason, nil)
	}
	return nil
}

func (a *actuator) infoEventf(managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet, reason, fmt string, args ...any) {
	a.recorder.Eventf(managedSeedSet, corev1.EventTypeNormal, reason, fmt, args...)
}

func (a *actuator) errorEventf(managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet, reason, fmt string, args ...any) {
	a.recorder.Eventf(managedSeedSet, corev1.EventTypeWarning, reason, fmt, args...)
}

func getPendingReplica(replicas []Replica, status *seedmanagementv1alpha1.ManagedSeedSetStatus) Replica {
	if status.PendingReplica == nil {
		return nil
	}
	for _, r := range replicas {
		if r.GetName() == status.PendingReplica.Name {
			return r
		}
	}
	return nil
}

func getPendingReplicaRetries(status *seedmanagementv1alpha1.ManagedSeedSetStatus, name string, reason seedmanagementv1alpha1.PendingReplicaReason) int32 {
	if status.PendingReplica != nil && status.PendingReplica.Name == name && status.PendingReplica.Reason == reason && status.PendingReplica.Retries != nil {
		return *status.PendingReplica.Retries
	}
	return 0
}

func updatePendingReplica(status *seedmanagementv1alpha1.ManagedSeedSetStatus, name string, reason seedmanagementv1alpha1.PendingReplicaReason, retries *int32) {
	if status.PendingReplica == nil || status.PendingReplica.Name != name || status.PendingReplica.Reason != reason || !reflect.DeepEqual(status.PendingReplica.Retries, retries) {
		status.PendingReplica = &seedmanagementv1alpha1.PendingReplica{
			Name:    name,
			Reason:  reason,
			Since:   Now(),
			Retries: retries,
		}
	}
}

func getNextOrdinal(replicas []Replica, status *seedmanagementv1alpha1.ManagedSeedSetStatus) int32 {
	// Replicas are sorted by ordinal, so the ordinal of the last replica is also the largest one
	if len(replicas) > 0 {
		if nextOrdinal := replicas[len(replicas)-1].GetOrdinal() + 1; nextOrdinal > status.NextReplicaNumber {
			return nextOrdinal
		}
	}
	return status.NextReplicaNumber
}

func replicaIsReady(r Replica) bool {
	return r.GetStatus() == StatusManagedSeedRegistered && r.IsSeedReady() && r.GetShootHealthStatus() == gardenerutils.ShootStatusHealthy
}

func debugReplica(r Replica, log logr.Logger) {
	log.Info("Replica", "objectKey", r.GetObjectKey(), "status", r.GetStatus().String(), "seedReady", r.IsSeedReady(), "shootHealthStatus", r.GetShootHealthStatus())
}

func replicaManagedSeedExists(status ReplicaStatus) bool {
	return status >= StatusManagedSeedPreparing
}

// ascendingOrdinal is a sort.Interface that sorts a list of replicas based on their ordinals.
// Replicas that have not been created by a ManagedSeedSet have an ordinal of -1, and are therefore pushed
// to the front of the list.
type ascendingOrdinal []Replica

func (ao ascendingOrdinal) Len() int {
	return len(ao)
}

func (ao ascendingOrdinal) Swap(i, j int) {
	ao[i], ao[j] = ao[j], ao[i]
}

func (ao ascendingOrdinal) Less(i, j int) bool {
	return ao[i].GetOrdinal() < ao[j].GetOrdinal()
}

// ascendingPriority is a sort.Interface that sorts a list of replicas based on their priority.
type ascendingPriority []Replica

func (ap ascendingPriority) Len() int {
	return len(ap)
}

func (ap ascendingPriority) Swap(i, j int) {
	ap[i], ap[j] = ap[j], ap[i]
}

func (ap ascendingPriority) Less(i, j int) bool {
	// First compare replica statuses
	// Replicas with "less advanced" status are considered lower priority
	if vi, vj := ap[i].GetStatus(), ap[j].GetStatus(); vi != vj {
		return vi < vj
	}

	// Then, compare replica seed readiness
	// Replicas with non-ready seeds are considered lower priority
	if vi, vj := ap[i].IsSeedReady(), ap[j].IsSeedReady(); vi != vj {
		return !vi
	}

	// Then, compare replica shoot health statuses
	// Replicas with "worse" status are considered lower priority
	if vi, vj := gardenerutils.ShootStatusValue(ap[i].GetShootHealthStatus()), gardenerutils.ShootStatusValue(ap[j].GetShootHealthStatus()); vi != vj {
		return vi < vj
	}

	// Finally, compare replica ordinals
	// Replicas with lower ordinals are considered lower priority
	return ap[i].GetOrdinal() < ap[j].GetOrdinal()
}
