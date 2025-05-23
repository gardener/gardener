// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"context"
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/health/utils"
	resourcemanagerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

// ControllerName is the name of the controller.
const ControllerName = "health"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, sourceCluster, targetCluster cluster.Cluster, clusterID string) error {
	if r.SourceClient == nil {
		r.SourceClient = sourceCluster.GetClient()
	}
	if r.TargetClient == nil {
		r.TargetClient = targetCluster.GetClient()
	}
	if r.TargetScheme == nil {
		r.TargetScheme = targetCluster.GetScheme()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		Watches(
			&resourcesv1alpha1.ManagedResource{},
			r.EnqueueCreateAndUpdate(),
			builder.WithPredicates(
				predicate.Or(
					resourcemanagerpredicate.ClassChangedPredicate(),
					// start health checks immediately after MR has been reconciled
					resourcemanagerpredicate.ConditionStatusChanged(resourcesv1alpha1.ResourcesApplied, resourcemanagerpredicate.DefaultConditionChange),
					resourcemanagerpredicate.NoLongerIgnored(),
				),
				resourcemanagerpredicate.NotIgnored(),
				r.ClassFilter,
			),
		).
		Build(r)
	if err != nil {
		return err
	}

	lock := sync.RWMutex{}
	watchedObjectGVKs := make(map[schema.GroupVersionKind]struct{})
	r.ensureWatchForGVK = func(gvk schema.GroupVersionKind, obj client.Object) error {
		// fast-check: have we already added watch for this GVK?
		lock.RLock()
		if _, ok := watchedObjectGVKs[gvk]; ok {
			lock.RUnlock()
			return nil
		}
		lock.RUnlock()

		// slow-check: two goroutines might concurrently call this func. If neither exited early, the first one added
		// the watch and the second one should return now.
		lock.Lock()
		defer lock.Unlock()
		if _, ok := watchedObjectGVKs[gvk]; ok {
			return nil
		}

		_, metadataOnly := obj.(*metav1.PartialObjectMetadata)
		c.GetLogger().Info("Adding new watch for GroupVersionKind", "groupVersionKind", gvk, "metadataOnly", metadataOnly)

		if err := c.Watch(source.Kind[client.Object](
			targetCluster.GetCache(),
			obj,
			handler.EnqueueRequestsFromMapFunc(utils.MapToOriginManagedResource(c.GetLogger(), clusterID)),
			utils.HealthStatusChanged(c.GetLogger()),
		)); err != nil {
			return fmt.Errorf("error starting watch for GVK %s: %w", gvk.String(), err)
		}

		watchedObjectGVKs[gvk] = struct{}{}
		return nil
	}

	return nil
}

// EnqueueCreateAndUpdate returns an event handler which only enqueues create and update events.
func (r *Reconciler) EnqueueCreateAndUpdate() handler.EventHandler {
	return &handler.Funcs{
		CreateFunc: func(_ context.Context, e event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      e.Object.GetName(),
				Namespace: e.Object.GetNamespace(),
			}})
		},
		UpdateFunc: func(_ context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      e.ObjectNew.GetName(),
				Namespace: e.ObjectNew.GetNamespace(),
			}})
		},
	}
}
