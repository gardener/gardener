// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/gardener/gardener/pkg/utils"
)

// EnqueueOnce is a source.Source that simply triggers the reconciler once by directly enqueueing an empty
// reconcile.Request.
var EnqueueOnce = source.Func(func(_ context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
	q.Add(reconcile.Request{})
	return nil
})

// HandleOnce triggers the reconciler once by calling 'Create' at the event handler with
// an empty event.CreateEvent.
type HandleOnce[object client.Object, request comparable] struct {
	Handler handler.TypedEventHandler[object, request]
}

// Start implements source.Source.
func (h *HandleOnce[object, request]) Start(ctx context.Context, q workqueue.TypedRateLimitingInterface[request]) error {
	h.Handler.Create(ctx, event.TypedCreateEvent[object]{}, q)
	return nil
}

// EnqueueAnonymously is a handler.EventHandler which enqueues a reconcile.Request without any namespace/name data.
var EnqueueAnonymously = handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
	return []reconcile.Request{{}}
})

// RandomDurationWithMetaDuration is an alias for `utils.RandomDurationWithMetaDuration`. Exposed for unit tests.
var RandomDurationWithMetaDuration = utils.RandomDurationWithMetaDuration

// EnqueueWithJitterDelay returns an EventHandler that enqueues objects with an optional jitter delay.
// getObservedGeneration must return the object's ObservedGeneration and true, or 0 and false if the object type is unexpected.
func EnqueueWithJitterDelay(
	getObservedGeneration func(client.Object) (int64, bool),
	jitterUpdates *bool,
	syncJitterPeriod *metav1.Duration,
) handler.EventHandler {
	reconcileRequest := func(obj client.Object) reconcile.Request {
		return reconcile.Request{NamespacedName: types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}}
	}

	addWithJitter := func(obj client.Object, q workqueue.TypedRateLimitingInterface[reconcile.Request], jitter bool) {
		if jitter {
			q.AddAfter(reconcileRequest(obj), RandomDurationWithMetaDuration(syncJitterPeriod))
		} else {
			q.Add(reconcileRequest(obj))
		}
	}

	return &handler.Funcs{
		CreateFunc: func(_ context.Context, e event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			observedGeneration, ok := getObservedGeneration(e.Object)
			if !ok {
				return
			}

			// Objects with deletion timestamp and newly created managed seed will be enqueued immediately.
			// Generation is 1 for newly created objects.
			if e.Object.GetDeletionTimestamp() != nil || e.Object.GetGeneration() == 1 {
				q.Add(reconcileRequest(e.Object))
				return
			}

			if e.Object.GetGeneration() != observedGeneration {
				addWithJitter(e.Object, q, ptr.Deref(jitterUpdates, false))
				return
			}

			// Spread reconciliation of  across the configured sync jitter
			// period to avoid overloading the gardener-apiserver
			addWithJitter(e.Object, q, true)
		},
		UpdateFunc: func(_ context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			observedGeneration, ok := getObservedGeneration(e.ObjectNew)
			if !ok {
				return
			}

			if e.ObjectNew.GetGeneration() == observedGeneration {
				return
			}

			// Objects with deletion timestamp and newly created objects will be enqueued immediately.
			// Generation is 1 for newly created objects.
			if e.ObjectNew.GetDeletionTimestamp() != nil || e.ObjectNew.GetGeneration() == 1 {
				q.Add(reconcileRequest(e.ObjectNew))
				return
			}

			addWithJitter(e.ObjectNew, q, ptr.Deref(jitterUpdates, false))
		},
		DeleteFunc: func(_ context.Context, e event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			if e.Object == nil {
				return
			}
			q.Add(reconcileRequest(e.Object))
		},
	}
}
