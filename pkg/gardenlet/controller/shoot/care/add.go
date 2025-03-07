// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"

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
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/utils"
)

// Request contains the information necessary to reconcile a shoot or managed resource
type Request struct {
	// NamespacedName is the name and namespace of the object to reconcile.
	types.NamespacedName
	IsManagedResource bool
}

// ControllerName is the name of this controller.
const ControllerName = "shoot-care"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	return builder.
		TypedControllerManagedBy[Request](mgr).
		Named(ControllerName).
		WithOptions(controller.TypedOptions[Request]{
			MaxConcurrentReconciles: ptr.Deref(r.Config.Controllers.ShootCare.ConcurrentSyncs, 0),
			// if going into exponential backoff, wait at most the configured sync period
			RateLimiter: workqueue.NewTypedWithMaxWaitRateLimiter(workqueue.DefaultTypedControllerRateLimiter[Request](), r.Config.Controllers.ShootCare.SyncPeriod.Duration),
		}).
		WatchesRawSource(
			source.TypedKind[client.Object](gardenCluster.GetCache(),
				&gardencorev1beta1.Shoot{},
				r.EventHandler(),
				r.ShootPredicate()),
		).
		WatchesRawSource(
			source.TypedKind[client.Object](r.SeedClientSet.Cache(),
				&resourcesv1alpha1.ManagedResource{},
				handler.TypedEnqueueRequestsFromMapFunc(r.MapManagedResource),
				predicateutils.ManagedResourceConditionsChanged()),
		).
		Complete(r)
}

// RandomDurationWithMetaDuration is an alias for utils.RandomDurationWithMetaDuration.
var RandomDurationWithMetaDuration = utils.RandomDurationWithMetaDuration

// EventHandler returns a handler for Shoot events.
func (r *Reconciler) EventHandler() handler.TypedEventHandler[client.Object, Request] {
	return &handler.TypedFuncs[client.Object, Request]{
		CreateFunc: func(_ context.Context, e event.CreateEvent, q workqueue.TypedRateLimitingInterface[Request]) {
			shoot, ok := e.Object.(*gardencorev1beta1.Shoot)
			if !ok {
				return
			}

			req := Request{NamespacedName: types.NamespacedName{
				Name:      e.Object.GetName(),
				Namespace: e.Object.GetNamespace(),
			}}

			if shoot.Generation == shoot.Status.ObservedGeneration {
				// spread shoot health checks across sync period to avoid checking on all Shoots roughly at the same
				// time after startup of the gardenlet
				q.AddAfter(req, RandomDurationWithMetaDuration(r.Config.Controllers.ShootCare.SyncPeriod))
				return
			}

			// don't add random duration for enqueueing new Shoots which have never been health checked yet
			q.Add(req)
		},
		UpdateFunc: func(_ context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[Request]) {
			q.Add(Request{NamespacedName: types.NamespacedName{
				Name:      e.ObjectNew.GetName(),
				Namespace: e.ObjectNew.GetNamespace(),
			}})
		},
	}
}

// ShootPredicate is a predicate which returns 'true' for create events, and for update events in case the shoot was
// successfully reconciled.
func (r *Reconciler) ShootPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			shoot, ok := e.ObjectNew.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}

			oldShoot, ok := e.ObjectOld.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}

			// re-evaluate shoot health status right after a reconciliation operation has succeeded
			return predicateutils.ReconciliationFinishedSuccessfully(oldShoot.Status.LastOperation, shoot.Status.LastOperation) || seedGotAssigned(oldShoot, shoot)
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

func seedGotAssigned(oldShoot, newShoot *gardencorev1beta1.Shoot) bool {
	return oldShoot.Status.SeedName == nil && newShoot.Status.SeedName != nil
}

// MapManagedResource is a mapper.MapFunc for mapping a ManagedResource to a request reference.
func (r *Reconciler) MapManagedResource(_ context.Context, mr client.Object) []Request {
	return []Request{{
		NamespacedName: types.NamespacedName{
			// avoid duplicate triggers by letting all managed resources of a shoot use the same
			// reconcile key
			Name:      "unused",
			Namespace: mr.GetNamespace(),
		},
		IsManagedResource: true,
	}}
}
