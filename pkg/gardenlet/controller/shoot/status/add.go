// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"maps"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
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

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
)

// ControllerName is the name of this controller.
const ControllerName = "shoot-status"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster, seedCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.SeedClient == nil {
		r.SeedClient = seedCluster.GetClient()
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: *r.Config.ConcurrentSyncs}).
		WatchesRawSource(source.Kind[client.Object](
			seedCluster.GetCache(),
			&extensionsv1alpha1.Worker{},
			handler.EnqueueRequestsFromMapFunc(r.MapWorkerToShoot(mgr.GetLogger().WithValues("controller", ControllerName))),
			WorkerStatusChangedPredicate(),
		)).
		Complete(r)
}

// MapWorkerToShoot is a handler.MapFunc for mapping an extensions.gardener.cloud/v1alpha1.Worker
// to the owning core.gardener.cloud/v1beta1.Shoot.
func (r *Reconciler) MapWorkerToShoot(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		if obj.GetDeletionTimestamp() != nil {
			return nil
		}

		shoot, err := extensions.GetShoot(ctx, r.SeedClient, obj.GetNamespace())
		if err != nil {
			log.Error(err, "Failed to get shoot from cluster", "shootTechnicalID", obj.GetNamespace())
			return nil
		}

		if shoot == nil {
			log.Info("Shoot is missing in cluster resource", "cluster", client.ObjectKey{Name: obj.GetNamespace()})
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: shoot.Name, Namespace: shoot.Namespace}}}
	}
}

// WorkerStatusChangedPredicate returns a predicate.Predicate that returns true if the status.inPlaceUpdates.workerPoolToHashMap of the Worker has changed.
func WorkerStatusChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			worker, ok := e.Object.(*extensionsv1alpha1.Worker)
			if !ok {
				return false
			}

			return worker.Status.InPlaceUpdates != nil && worker.Status.InPlaceUpdates.WorkerPoolToHashMap != nil
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldWorker, ok := e.ObjectOld.(*extensionsv1alpha1.Worker)
			if !ok {
				return false
			}

			newWorker, ok := e.ObjectNew.(*extensionsv1alpha1.Worker)
			if !ok {
				return false
			}

			var (
				newWorkerPoolToHashMap map[string]string
				oldWorkerPoolToHashMap map[string]string
			)

			if oldWorker.Status.InPlaceUpdates != nil {
				oldWorkerPoolToHashMap = oldWorker.Status.InPlaceUpdates.WorkerPoolToHashMap
			}

			if newWorker.Status.InPlaceUpdates != nil {
				newWorkerPoolToHashMap = newWorker.Status.InPlaceUpdates.WorkerPoolToHashMap
			}

			return !maps.Equal(oldWorkerPoolToHashMap, newWorkerPoolToHashMap)
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}
