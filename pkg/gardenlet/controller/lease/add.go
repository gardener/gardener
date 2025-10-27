// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lease

import (
	"time"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster cluster.Cluster, controllerNamePrefix string, predicates ...predicate.Predicate) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(controllerNamePrefix + "-lease").
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			RateLimiter:             workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](time.Millisecond, 2*time.Second),
			ReconciliationTimeout:   time.Duration(r.LeaseResyncSeconds) * time.Second,
		}).
		WatchesRawSource(source.Kind[client.Object](
			gardenCluster.GetCache(),
			r.NewObjectFunc(),
			&handler.EnqueueRequestForObject{},
			append([]predicate.Predicate{predicateutils.ForEventTypes(predicateutils.Create)}, predicates...)...,
		)).
		Complete(r)
}
