// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenletlifecycle

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "gardenlet-lifecycle"

// Request contains the namespace/name of the object as well as information whether it is a self-hosted Shoot.
type Request struct {
	reconcile.Request

	IsSelfHostedShoot bool
}

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	return builder.
		TypedControllerManagedBy[Request](mgr).
		Named(ControllerName).
		Watches(&gardencorev1beta1.Seed{}, r.EventHandler(), builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create))).
		// TODO(rfranzke): In the next commit, we add a predicate that filters for self-hosted shoots only.
		Watches(&gardencorev1beta1.Shoot{}, r.EventHandler(), builder.WithPredicates(predicateutils.ForEventTypes(predicateutils.Create))).
		WithOptions(controller.TypedOptions[Request]{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
			ReconciliationTimeout:   r.Config.SyncPeriod.Duration,
		}).
		Complete(r)
}

// EventHandler returns a handler for events.
func (r *Reconciler) EventHandler() handler.TypedEventHandler[client.Object, Request] {
	return &handler.TypedFuncs[client.Object, Request]{
		CreateFunc: func(_ context.Context, e event.TypedCreateEvent[client.Object], q workqueue.TypedRateLimitingInterface[Request]) {
			if e.Object != nil {
				_, isSelfHostedShoot := e.Object.(*gardencorev1beta1.Shoot)
				q.Add(Request{
					Request:           reconcile.Request{NamespacedName: types.NamespacedName{Name: e.Object.GetName(), Namespace: e.Object.GetNamespace()}},
					IsSelfHostedShoot: isSelfHostedShoot,
				})
			}
		},
	}
}
