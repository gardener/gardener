// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "shoot-state"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster, seedCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.SeedClient == nil {
		r.SeedClient = seedCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: *r.Config.ConcurrentSyncs,
			ReconciliationTimeout:   controllerutils.DefaultReconciliationTimeout,
		}).
		WatchesRawSource(
			source.Kind[client.Object](gardenCluster.GetCache(),
				&gardencorev1beta1.Shoot{},
				&handler.EnqueueRequestForObject{},
				predicate.Or(r.SeedNameChangedPredicate(), r.ShootCreationSucceededPredicate())),
		).
		Complete(r)
}

// SeedNameChangedPredicate returns a predicate which returns true for all events except updates - here it only returns
// true when the seed name changed.
func (r *Reconciler) SeedNameChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			newShoot, ok := updateEvent.ObjectNew.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}

			oldShoot, ok := updateEvent.ObjectOld.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}

			return ptr.Deref(newShoot.Spec.SeedName, "") != ptr.Deref(oldShoot.Spec.SeedName, "")
		},
	}
}

// ShootCreationSucceededPredicate returns a predicate which returns true for update events where the Shoot's
// initial Create operation just transitioned from Processing to Succeeded.
func (r *Reconciler) ShootCreationSucceededPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool { return false },
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			oldShoot, ok := updateEvent.ObjectOld.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}

			newShoot, ok := updateEvent.ObjectNew.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}

			return predicateutils.CreationSucceeded(oldShoot.Status.LastOperation, newShoot.Status.LastOperation)
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}
