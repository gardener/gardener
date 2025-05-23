// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseed

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

// ControllerName is the name of this controller.
const ControllerName = "managedseed"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(
	mgr manager.Manager,
	gardenCluster cluster.Cluster,
	seedCluster cluster.Cluster,
) error {
	if r.GardenConfig == nil {
		r.GardenConfig = gardenCluster.GetConfig()
	}
	if r.GardenAPIReader == nil {
		r.GardenAPIReader = gardenCluster.GetAPIReader()
	}
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.SeedClient == nil {
		r.SeedClient = seedCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.Recorder == nil {
		r.Recorder = gardenCluster.GetEventRecorderFor(ControllerName + "-controller")
	}
	if r.GardenNamespaceGarden == "" {
		r.GardenNamespaceGarden = v1beta1constants.GardenNamespace
	}
	if r.GardenNamespaceShoot == "" {
		r.GardenNamespaceShoot = v1beta1constants.GardenNamespace
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.Controllers.ManagedSeed.ConcurrentSyncs, 0),
		}).
		WatchesRawSource(source.Kind[client.Object](
			gardenCluster.GetCache(),
			&seedmanagementv1alpha1.ManagedSeed{},
			r.EnqueueWithJitterDelay(),
			&predicate.GenerationChangedPredicate{},
		)).
		WatchesRawSource(source.Kind[client.Object](
			gardenCluster.GetCache(),
			&gardencorev1beta1.Seed{},
			handler.EnqueueRequestsFromMapFunc(r.MapSeedToManagedSeed),
			r.SeedPredicate(),
		)).
		Complete(r)
}

// SeedPredicate returns true when the Seed is a ManagedSeed controlled by this gardenlet. ManagedSeeds always have two
// `name.seed.gardener.cloud/` labels, and since the cache for Seeds is already limited on manager.Manager level to only
// contain Seeds relevant for this gardenlet, we can make this simple check here.
func (r *Reconciler) SeedPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		count := 0
		for key := range object.GetLabels() {
			if strings.HasPrefix(key, v1beta1constants.LabelPrefixSeedName) {
				count++
			}
		}
		return count > 1
	})
}

// MapSeedToManagedSeed is a handler.MapFunc for mapping a Seed to the owning ManagedSeed.
func (r *Reconciler) MapSeedToManagedSeed(_ context.Context, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: r.GardenNamespaceGarden, Name: obj.GetName()}}}
}

func reconcileRequest(obj client.Object) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}}
}

// RandomDurationWithMetaDuration is an alias for `utils.RandomDurationWithMetaDuration`. Exposed for unit tests.
var RandomDurationWithMetaDuration = utils.RandomDurationWithMetaDuration

// EnqueueWithJitterDelay returns handler.Funcs which enqueues the object with a random Jitter duration when the JitterUpdate
// is enabled in ManagedSeed controller configuration.
// All other events are normally enqueued.
func (r *Reconciler) EnqueueWithJitterDelay() handler.EventHandler {
	return &handler.Funcs{
		CreateFunc: func(_ context.Context, evt event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			managedSeed, ok := evt.Object.(*seedmanagementv1alpha1.ManagedSeed)
			if !ok {
				return
			}

			generationChanged := managedSeed.Generation != managedSeed.Status.ObservedGeneration

			// Managed seed with deletion timestamp and newly created managed seed will be enqueued immediately.
			// Generation is 1 for newly created objects.
			if managedSeed.DeletionTimestamp != nil || managedSeed.Generation == 1 {
				q.Add(reconcileRequest(evt.Object))
				return
			}

			if generationChanged {
				if *r.Config.Controllers.ManagedSeed.JitterUpdates {
					q.AddAfter(reconcileRequest(evt.Object), RandomDurationWithMetaDuration(r.Config.Controllers.ManagedSeed.SyncJitterPeriod))
				} else {
					q.Add(reconcileRequest(evt.Object))
				}
				return
			}
			// Spread reconciliation of managed seeds (including gardenlet updates/rollouts) across the configured sync jitter
			// period to avoid overloading the gardener-apiserver if all gardenlets in all managed seeds are (re)starting
			// roughly at the same time.
			q.AddAfter(reconcileRequest(evt.Object), RandomDurationWithMetaDuration(r.Config.Controllers.ManagedSeed.SyncJitterPeriod))
		},
		UpdateFunc: func(_ context.Context, evt event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			managedSeed, ok := evt.ObjectNew.(*seedmanagementv1alpha1.ManagedSeed)
			if !ok {
				return
			}

			if managedSeed.Generation == managedSeed.Status.ObservedGeneration {
				return
			}

			// Managed seed with deletion timestamp and newly created managed seed will be enqueued immediately.
			// Generation is 1 for newly created objects.
			if managedSeed.DeletionTimestamp != nil || managedSeed.Generation == 1 {
				q.Add(reconcileRequest(evt.ObjectNew))
				return
			}

			if ptr.Deref(r.Config.Controllers.ManagedSeed.JitterUpdates, false) {
				q.AddAfter(reconcileRequest(evt.ObjectNew), RandomDurationWithMetaDuration(r.Config.Controllers.ManagedSeed.SyncJitterPeriod))
			} else {
				q.Add(reconcileRequest(evt.ObjectNew))
			}
		},
		DeleteFunc: func(_ context.Context, evt event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			if evt.Object == nil {
				return
			}
			q.Add(reconcileRequest(evt.Object))
		},
	}
}
