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
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	operationshoot "github.com/gardener/gardener/pkg/operation/shoot"
	contextutil "github.com/gardener/gardener/pkg/utils/context"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// ShootPredicate returns the predicate for Shoot events.
// ShootPredicate reacts only on 'CREATE','UPDATE' and 'DELETE' events. It returns true when shoot is deleted.
// For updates,returns true if:
// - shoot's health status changed.
// - shoot belongs to the pending replica and it progressed from the state that caused the replica to be pending.
func (r *Reconciler) ShootPredicate() predicate.Predicate {
	return &shootPredicate{}
}

type shootPredicate struct {
	ctx    context.Context
	reader client.Reader
}

func (p *shootPredicate) InjectStopChannel(stopChan <-chan struct{}) error {
	p.ctx = contextutil.FromStopChannel(stopChan)
	return nil
}

func (p *shootPredicate) InjectClient(client client.Client) error {
	p.reader = client
	return nil
}

func (p *shootPredicate) Create(_ event.CreateEvent) bool { return true }

func (p *shootPredicate) Update(e event.UpdateEvent) bool {
	shoot, ok := e.ObjectNew.(*gardencorev1beta1.Shoot)
	if !ok {
		return false
	}

	oldShoot, ok := e.ObjectOld.(*gardencorev1beta1.Shoot)
	if !ok {
		return false
	}

	setName := shoot.GetOwnerReferences()[0].Name
	set := &seedmanagementv1alpha1.ManagedSeedSet{}
	err := p.reader.Get(p.ctx, kutil.Key(shoot.Namespace, setName), set)
	if err != nil {
		return false
	}

	if set.Status.PendingReplica == nil || set.Status.PendingReplica.Name != shoot.Name {
		return false
	}

	if shootHealthStatus(shoot) != shootHealthStatus(oldShoot) {
		return true
	}

	switch set.Status.PendingReplica.Reason {
	case seedmanagementv1alpha1.ShootReconcilingReason:
		return shootReconcileFailed(shoot) || shootReconcileSucceeded(shoot) || shoot.DeletionTimestamp != nil
	case seedmanagementv1alpha1.ShootDeletingReason:
		return shootDeleteFailed(shoot)
	case seedmanagementv1alpha1.ShootReconcileFailedReason:
		return !shootReconcileFailed(shoot)
	case seedmanagementv1alpha1.ShootDeleteFailedReason:
		return !shootDeleteFailed(shoot)
	case seedmanagementv1alpha1.ShootNotHealthyReason:
		return shootHealthStatus(shoot) == operationshoot.StatusHealthy
	default:
		return false
	}
}

func (p *shootPredicate) Delete(_ event.DeleteEvent) bool { return true }

func (p *shootPredicate) Generic(_ event.GenericEvent) bool { return false }

func (c *Controller) filterManagedSeed(obj, oldObj, controller client.Object, deleted bool) bool {
	managedSeed, ok := obj.(*seedmanagementv1alpha1.ManagedSeed)
	if !ok {
		return false
	}
	set, ok := controller.(*seedmanagementv1alpha1.ManagedSeedSet)
	if !ok {
		return false
	}

	// If the managed seed was deleted, return true
	if oldObj != nil {
		oldManagedSeed, ok := oldObj.(*seedmanagementv1alpha1.ManagedSeed)
		if !ok {
			return false
		}
		if !reflect.DeepEqual(managedSeed.DeletionTimestamp, oldManagedSeed.DeletionTimestamp) {
			c.log.V(1).Info("ManagedSeed was deleted", "managedSeed", client.ObjectKeyFromObject(managedSeed))
			return true
		}
	}

	// Return true only if the managed seed belongs to the pending replica and it progressed from the state
	// that caused the replica to be pending
	if set.Status.PendingReplica == nil || set.Status.PendingReplica.Name != managedSeed.Name {
		return false
	}
	switch set.Status.PendingReplica.Reason {
	case seedmanagementv1alpha1.ManagedSeedPreparingReason:
		return managedSeedRegistered(managedSeed) || managedSeed.DeletionTimestamp != nil
	case seedmanagementv1alpha1.ManagedSeedDeletingReason:
		return deleted
	default:
		return false
	}
}

func (c *Controller) filterSeed(obj, oldObj, controller client.Object, _ bool) bool {
	seed, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return false
	}
	set, ok := controller.(*seedmanagementv1alpha1.ManagedSeedSet)
	if !ok {
		return false
	}

	// If the seed readiness changed, return true
	if oldObj != nil {
		oldSeed, ok := oldObj.(*gardencorev1beta1.Seed)
		if !ok {
			return false
		}
		if seedReady(seed) != seedReady(oldSeed) {
			c.log.V(1).Info("Seed readiness changed", "seed", client.ObjectKeyFromObject(seed))
			return true
		}
	}

	// Return true only if the seed belongs to the pending replica and it progressed from the state
	// that caused the replica to be pending
	if set.Status.PendingReplica == nil || set.Status.PendingReplica.Name != seed.Name {
		return false
	}
	switch set.Status.PendingReplica.Reason {
	case seedmanagementv1alpha1.SeedNotReadyReason:
		return seedReady(seed)
	default:
		return false
	}
}
