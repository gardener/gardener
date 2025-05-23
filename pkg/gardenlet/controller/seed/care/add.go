// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "seed-care"

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
			MaxConcurrentReconciles: 1,
			// if going into exponential backoff, wait at most the configured sync period
			RateLimiter: workqueue.NewTypedWithMaxWaitRateLimiter(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](), r.Config.SyncPeriod.Duration),
		}).
		WatchesRawSource(source.Kind[client.Object](
			gardenCluster.GetCache(),
			&gardencorev1beta1.Seed{},
			&handler.EnqueueRequestForObject{},
			predicateutils.HasName(r.SeedName),
			r.SeedPredicate(),
		)).
		WatchesRawSource(source.Kind[client.Object](
			seedCluster.GetCache(),
			&resourcesv1alpha1.ManagedResource{},
			handler.EnqueueRequestsFromMapFunc(r.MapManagedResourceToSeed),
			r.IsSystemComponent(),
			predicateutils.ManagedResourceConditionsChanged(),
		)).
		Complete(r)
}

// SeedPredicate is a predicate which returns 'true' for create events, and for update events in case the seed was
// successfully bootstrapped.
func (r *Reconciler) SeedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			seed, ok := e.ObjectNew.(*gardencorev1beta1.Seed)
			if !ok {
				return false
			}

			oldSeed, ok := e.ObjectOld.(*gardencorev1beta1.Seed)
			if !ok {
				return false
			}

			return predicateutils.ReconciliationFinishedSuccessfully(oldSeed.Status.LastOperation, seed.Status.LastOperation)
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

// IsSystemComponent returns a predicate which evaluates to true in case the gardener.cloud/role=system-component label
// is present.
func (r *Reconciler) IsSystemComponent() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetLabels()[v1beta1constants.GardenRole] == v1beta1constants.GardenRoleSeedSystemComponent
	})
}

// MapManagedResourceToSeed is a handler.MapFunc for mapping a ManagedResource to the owning Seed.
func (r *Reconciler) MapManagedResourceToSeed(_ context.Context, _ client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: r.SeedName}}}
}
