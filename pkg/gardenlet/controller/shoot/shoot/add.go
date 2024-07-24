// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot/helper"
)

// ControllerName is the name of this controller.
const ControllerName = "shoot"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = gardenCluster.GetEventRecorderFor(ControllerName + "-controller")
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	// It's not possible to call builder.Build() without adding atleast one watch, and without this, we can't get the controller logger.
	// Hence, we have to build up the controller manually.
	c, err := controller.New(
		ControllerName,
		mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: ptr.Deref(r.Config.Controllers.Shoot.ConcurrentSyncs, 0),
		},
	)
	if err != nil {
		return err
	}

	return c.Watch(
		source.Kind(gardenCluster.GetCache(),
			&gardencorev1beta1.Shoot{},
			r.EventHandler(c.GetLogger()),
			&predicate.GenerationChangedPredicate{}),
	)
}

// CalculateControllerInfos is exposed for testing
var CalculateControllerInfos = helper.CalculateControllerInfos

// EventHandler returns an event handler.
func (r *Reconciler) EventHandler(log logr.Logger) handler.EventHandler {
	return &handler.Funcs{
		CreateFunc: func(_ context.Context, e event.CreateEvent, q workqueue.RateLimitingInterface) {
			shoot, ok := e.Object.(*gardencorev1beta1.Shoot)
			if !ok {
				return
			}

			enqueueAfter := CalculateControllerInfos(shoot, r.Clock, *r.Config.Controllers.Shoot).EnqueueAfter
			nextReconciliation := r.Clock.Now().UTC().Add(enqueueAfter)

			log.Info("Scheduling next reconciliation for Shoot",
				"namespace", shoot.Namespace, "name", shoot.Name,
				"enqueueAfter", enqueueAfter, "nextReconciliation", nextReconciliation)

			q.AddAfter(reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      e.Object.GetName(),
				Namespace: e.Object.GetNamespace(),
			}}, enqueueAfter)
		},
		UpdateFunc: func(_ context.Context, e event.UpdateEvent, q workqueue.RateLimitingInterface) {
			req := reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      e.ObjectNew.GetName(),
				Namespace: e.ObjectNew.GetNamespace(),
			}}

			// If the shoot's deletion timestamp is set then we want to forget about the potentially established exponential
			// backoff and enqueue it faster.
			if e.ObjectOld.GetDeletionTimestamp() == nil && e.ObjectNew.GetDeletionTimestamp() != nil {
				q.Forget(req)
			}

			q.Add(req)
		},
	}
}
