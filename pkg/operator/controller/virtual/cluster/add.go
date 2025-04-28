// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of this controller.
const ControllerName = "virtual-cluster-registrar"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, channel <-chan event.TypedGenericEvent[*rest.Config]) error {
	if r.Manager == nil {
		r.Manager = mgr
	}

	return builder.
		TypedControllerManagedBy[Request](mgr).
		Named(ControllerName).
		WatchesRawSource(source.TypedChannel(channel, r.EventHandler())).
		WithOptions(controller.TypedOptions[Request]{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}

// EventHandler returns a handler for rest.Config events.
func (r *Reconciler) EventHandler() handler.TypedEventHandler[*rest.Config, Request] {
	return &handler.TypedFuncs[*rest.Config, Request]{
		GenericFunc: func(_ context.Context, e event.TypedGenericEvent[*rest.Config], q workqueue.TypedRateLimitingInterface[Request]) {
			if e.Object != nil {
				q.Add(Request{RESTConfig: e.Object})
			}
		},
	}
}
