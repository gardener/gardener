// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package managedseedset

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	operationshoot "github.com/gardener/gardener/pkg/operation/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Actuator acts upon ManagedSeedSet resources.
type Actuator interface {
	// Reconcile reconciles ManagedSeedSet creation, update, or deletion.
	Reconcile(context.Context, *seedmanagementv1alpha1.ManagedSeedSet) (*seedmanagementv1alpha1.ManagedSeedSetStatus, bool, error)
}

// actuator is a concrete implementation of Actuator.
type actuator struct {
	gardenClient   kubernetes.Interface
	replicaGetter  ReplicaGetter
	replicaFactory ReplicaFactory
	cfg            *config.ManagedSeedSetControllerConfiguration
	recorder       record.EventRecorder
	logger         *logrus.Logger
}

// NewActuator creates and returns a new Actuator with the given parameters.
func NewActuator(
	gardenClient kubernetes.Interface,
	replicaGetter ReplicaGetter,
	replicaFactory ReplicaFactory,
	cfg *config.ManagedSeedSetControllerConfiguration,
	recorder record.EventRecorder,
	logger *logrus.Logger,
) Actuator {
	return &actuator{
		gardenClient:   gardenClient,
		replicaFactory: replicaFactory,
		replicaGetter:  replicaGetter,
		cfg:            cfg,
		recorder:       recorder,
		logger:         logger,
	}
}

// Now returns the current local time. Exposed for testing.
var Now = metav1.Now

// Reconcile reconciles ManagedSeedSet creation or update.
func (a *actuator) Reconcile(ctx context.Context, set *seedmanagementv1alpha1.ManagedSeedSet) (status *seedmanagementv1alpha1.ManagedSeedSetStatus, removeFinalizer bool, err error) {
	// Initialize status
	status = set.Status.DeepCopy()
	status.ObservedGeneration = set.Generation

	defer func() {
		if err != nil {
			a.errorEventf(set, gardencorev1beta1.EventReconcileError, err.Error())
		}
	}()

	// Get replicas
	replicas, err := a.replicaGetter.GetReplicas(ctx, set)
	if err != nil {
		return status, false, err
	}

	// Sort replicas by ascending ordinal
	sort.Sort(ascendingOrdinal(replicas))

	// Get the pending replica, if any
	pendingReplica := getPendingReplica(replicas, status)

	// Determine ready, postponed, and deletable replicas
	var readyReplicas, postponedReplicas, deletableReplicas []Replica
	var debug []string
	for _, r := range replicas {
		if replicaIsReady(r) {
			readyReplicas = append(readyReplicas, r)
		} else if r != pendingReplica {
			postponedReplicas = append(postponedReplicas, r)
		}
		if r.IsDeletable() {
			deletableReplicas = append(deletableReplicas, r)
		}
		debug = append(debug, replicaDebugString(r))
	}
	a.getLogger(set).Debugf("Replicas: %s", debug)

	// Update replicas and readyReplicas in status
	status.Replicas = int32(len(replicas))
	status.ReadyReplicas = int32(len(readyReplicas))

	// Determine the actual and target replica counts
	count := len(replicas)
	targetCount := 0
	if set.DeletionTimestamp == nil {
		targetCount = int(*set.Spec.Replicas)
	}

	// Determine whether scaling out or in
	scalingOut, scalingIn := count < targetCount, count > targetCount

	// Reconcile the pending replica, if any
	if pendingReplica != nil {
		if pending, err := a.reconcileReplica(ctx, set, status, pendingReplica, scalingIn); err != nil || pending {
			return status, false, err
		}
	}

	switch {
	case scalingOut:
		// Initialize a new replica and create its shoot
		ordinal := getNextOrdinal(replicas, status)
		if err := a.createReplica(ctx, set, status, ordinal); err != nil {
			return status, false, err
		}

		// Increment Replicas and NextReplicaNumber in status
		status.Replicas++
		status.NextReplicaNumber = int32(ordinal + 1)

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
		if err := a.deleteReplica(ctx, set, status, r); err != nil {
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
		if pending, err := a.reconcileReplica(ctx, set, status, r, scalingIn); err != nil || pending {
			return status, false, err
		}
	}

	a.getLogger(set).Debugf("Nothing to do")
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
	set *seedmanagementv1alpha1.ManagedSeedSet,
	status *seedmanagementv1alpha1.ManagedSeedSetStatus,
	r Replica,
	scalingIn bool,
) (bool, error) {
	replicaStatus := r.GetStatus()

	switch {
	case replicaStatus == StatusShootReconcileFailed && !scalingIn:
		// This replica's shoot reconciliation has failed, retry it if max retries is not yet reached
		retries := getPendingReplicaRetries(status, r.GetName(), seedmanagementv1alpha1.ShootReconcilingReason)
		if int(retries) < *a.cfg.MaxShootRetries {
			a.infoEventf(set, EventRetryingShootReconciliation, "Retrying shoot %s reconciliation", r.GetFullName())
			if err := r.RetryShoot(ctx, a.gardenClient.Client()); err != nil {
				return false, err
			}
			updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootReconcilingReason, pointer.Int32(retries+1))
		} else {
			a.infoEventf(set, EventNotRetryingShootReconciliation, "Not retrying shoot %s reconciliation since max retries has been reached", r.GetFullName())
			updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootReconcileFailedReason, &retries)
		}
		return true, nil

	case replicaStatus == StatusShootDeleteFailed:
		// This replica's shoot deletion has failed, retry it if max retries is not yet reached
		retries := getPendingReplicaRetries(status, r.GetName(), seedmanagementv1alpha1.ShootDeletingReason)
		if int(retries) < *a.cfg.MaxShootRetries {
			a.infoEventf(set, EventRetryingShootDeletion, "Retrying shoot %s deletion", r.GetFullName())
			if err := r.RetryShoot(ctx, a.gardenClient.Client()); err != nil {
				return false, err
			}
			updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootDeletingReason, pointer.Int32(retries+1))
		} else {
			a.infoEventf(set, EventNotRetryingShootDeletion, "Not retrying shoot %s deletion since max retries has been reached", r.GetFullName())
			updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootDeleteFailedReason, &retries)
		}
		return true, nil

	case replicaStatus == StatusShootReconciling && !scalingIn:
		// This replica's shoot is reconciling, wait for it to be reconciled before moving to the next replica
		a.infoEventf(set, EventWaitingForShootReconciled, "Waiting for shoot %s to be reconciled", r.GetFullName())
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootReconcilingReason, nil)
		return true, nil

	case replicaStatus == StatusShootDeleting:
		// This replica's shoot is deleting, wait for it to be deleted before moving to the next replica
		a.infoEventf(set, EventWaitingForShootDeleted, "Waiting for shoot %s to be deleted", r.GetFullName())
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootDeletingReason, nil)
		return true, nil

	case replicaStatus == StatusShootReconciled:
		// This replica's shoot is fully reconciled and its managed seed doesn't exist
		// If not scaling in, create its managed seed, otherwise delete its shoot
		if !scalingIn {
			a.infoEventf(set, EventCreatingManagedSeed, "Creating managed seed %s", r.GetFullName())
			if err := r.CreateManagedSeed(ctx, a.gardenClient.Client()); err != nil {
				return false, err
			}
			updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil)
		} else {
			a.infoEventf(set, EventDeletingShoot, "Deleting shoot %s", r.GetFullName())
			if err := r.DeleteShoot(ctx, a.gardenClient.Client()); err != nil {
				return false, err
			}
			updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootDeletingReason, nil)
		}
		return true, nil

	case replicaStatus == StatusManagedSeedPreparing && !scalingIn:
		// This replica's managed seed is preparing, wait for the it to be registered before moving to the next replica
		a.infoEventf(set, EventWaitingForManagedSeedRegistered, "Waiting for managed seed %s to be registered", r.GetFullName())
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil)
		return true, nil

	case replicaStatus == StatusManagedSeedDeleting:
		// This replica's managed seed is deleting, wait for it to be deleted before moving to the next replica
		a.infoEventf(set, EventWaitingForManagedSeedDeleted, "Waiting for managed seed %s to be deleted", r.GetFullName())
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ManagedSeedDeletingReason, nil)
		return true, nil

	case !r.IsSeedReady() && !scalingIn:
		// This replica's seed is not ready, wait for it to be ready before moving to the next replica
		a.infoEventf(set, EventWaitingForSeedReady, "Waiting for seed %s to be ready", r.GetName())
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.SeedNotReadyReason, nil)
		return true, nil

	case r.GetShootHealthStatus() != operationshoot.StatusHealthy && !scalingIn:
		// This replica's shoot is not healthy, wait for it to be healthy before moving to the next replica
		a.infoEventf(set, EventWaitingForShootHealthy, "Waiting for shoot %s to be healthy", r.GetFullName())
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootNotHealthyReason, nil)
		return true, nil
	}

	return false, nil
}

func (a *actuator) createReplica(
	ctx context.Context,
	set *seedmanagementv1alpha1.ManagedSeedSet,
	status *seedmanagementv1alpha1.ManagedSeedSetStatus,
	ordinal int,
) error {
	r := a.replicaFactory.NewReplica(set, nil, nil, nil, false)
	a.infoEventf(set, EventCreatingShoot, "Creating shoot %s", getFullName(set, ordinal))
	if err := r.CreateShoot(ctx, a.gardenClient.Client(), ordinal); err != nil {
		return err
	}
	updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootReconcilingReason, nil)
	return nil
}

func (a *actuator) deleteReplica(
	ctx context.Context,
	set *seedmanagementv1alpha1.ManagedSeedSet,
	status *seedmanagementv1alpha1.ManagedSeedSetStatus,
	r Replica,
) error {
	if replicaManagedSeedExists(r.GetStatus()) {
		a.infoEventf(set, EventDeletingManagedSeed, "Deleting managed seed %s", r.GetFullName())
		if err := r.DeleteManagedSeed(ctx, a.gardenClient.Client()); err != nil {
			return err
		}
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ManagedSeedDeletingReason, nil)
	} else {
		a.infoEventf(set, EventDeletingShoot, "Deleting shoot %s", r.GetFullName())
		if err := r.DeleteShoot(ctx, a.gardenClient.Client()); err != nil {
			return err
		}
		updatePendingReplica(status, r.GetName(), seedmanagementv1alpha1.ShootDeletingReason, nil)
	}
	return nil
}

func (a *actuator) infoEventf(set *seedmanagementv1alpha1.ManagedSeedSet, reason, fmt string, args ...interface{}) {
	a.recorder.Eventf(set, corev1.EventTypeNormal, reason, fmt, args...)
	a.getLogger(set).Infof(fmt, args...)
}

func (a *actuator) errorEventf(set *seedmanagementv1alpha1.ManagedSeedSet, reason, fmt string, args ...interface{}) {
	a.recorder.Eventf(set, corev1.EventTypeWarning, reason, fmt, args...)
	a.getLogger(set).Errorf(fmt, args...)
}

func (a *actuator) getLogger(set *seedmanagementv1alpha1.ManagedSeedSet) *logrus.Entry {
	return logger.NewFieldLogger(a.logger, "managedSeedSet", kutil.ObjectName(set))
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

func getNextOrdinal(replicas []Replica, status *seedmanagementv1alpha1.ManagedSeedSetStatus) int {
	// Replicas are sorted by ordinal, so the ordinal of the last replica is also the largest one
	if len(replicas) > 0 {
		if nextOrdinal := replicas[len(replicas)-1].GetOrdinal() + 1; nextOrdinal > int(status.NextReplicaNumber) {
			return nextOrdinal
		}
	}
	return int(status.NextReplicaNumber)
}

func replicaIsReady(r Replica) bool {
	return r.GetStatus() == StatusManagedSeedRegistered && r.IsSeedReady() && r.GetShootHealthStatus() == operationshoot.StatusHealthy
}

func replicaDebugString(r Replica) string {
	return fmt.Sprintf("%s:%s,%t,%s", r.GetFullName(), r.GetStatus().String(), r.IsSeedReady(), r.GetShootHealthStatus())
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
	if vi, vj := operationshoot.StatusValue(ap[i].GetShootHealthStatus()), operationshoot.StatusValue(ap[j].GetShootHealthStatus()); vi != vj {
		return vi < vj
	}

	// Finally, compare replica ordinals
	// Replicas with lower ordinals are considered lower priority
	return ap[i].GetOrdinal() < ap[j].GetOrdinal()
}
