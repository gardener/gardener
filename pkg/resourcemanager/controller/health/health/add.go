// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"k8s.io/utils/pointer"
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
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/health/utils"
	resourcemanagerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

// ControllerName is the name of the controller.
const ControllerName = "health"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager, sourceCluster, targetCluster cluster.Cluster, targetCacheDisabled bool, clusterID string) error {
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
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
		}).
		Watches(
			&source.Kind{Type: &resourcesv1alpha1.ManagedResource{}},
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

	if targetCacheDisabled {
		// if the target cache is disable, we don't want to start additional informers
		r.ensureWatchForGVK = func(gvk schema.GroupVersionKind, obj client.Object) error {
			return nil
		}
	} else {
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

			if err := c.Watch(
				source.NewKindWithCache(obj, targetCluster.GetCache()),
				mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), utils.MapToOriginManagedResource(clusterID), mapper.UpdateWithNew, c.GetLogger()),
				utils.HealthStatusChanged(c.GetLogger()),
			); err != nil {
				return fmt.Errorf("error starting watch for GVK %s: %w", gvk.String(), err)
			}

			watchedObjectGVKs[gvk] = struct{}{}
			return nil
		}
	}

	return nil
}

// EnqueueCreateAndUpdate returns an event handler which only enqueues create and update events.
func (r *Reconciler) EnqueueCreateAndUpdate() handler.EventHandler {
	return &handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      e.Object.GetName(),
				Namespace: e.Object.GetNamespace(),
			}})
		},
		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      e.ObjectNew.GetName(),
				Namespace: e.ObjectNew.GetNamespace(),
			}})
		},
	}
}
