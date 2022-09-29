// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
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
	resourceshelper "github.com/gardener/gardener/pkg/apis/resources/v1alpha1/helper"
	managerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

// ControllerName is the name of the health controller.
const ControllerName = "health"

// defaultControllerConfig is the default config for the controller.
var defaultControllerConfig ControllerConfig

// ControllerOptions are options for adding the controller to a Manager.
type ControllerOptions struct {
	maxConcurrentWorkers int
	syncPeriod           time.Duration
}

// ControllerConfig is the completed configuration for the controller.
type ControllerConfig struct {
	MaxConcurrentWorkers int
	SyncPeriod           time.Duration

	ClassFilter         managerpredicate.ClassFilter
	TargetCluster       cluster.Cluster
	TargetCacheDisabled bool
	ClusterID           string
}

// AddToManagerWithOptions adds the controller to a Manager with the given config.
func AddToManagerWithOptions(mgr manager.Manager, conf ControllerConfig) error {
	// setup main health reconciler
	healthReconciler := &reconciler{
		syncPeriod:   conf.SyncPeriod,
		classFilter:  &conf.ClassFilter,
		targetClient: conf.TargetCluster.GetClient(),
		targetScheme: conf.TargetCluster.GetScheme(),
	}

	healthController, err := controller.New(ControllerName, mgr, controller.Options{
		MaxConcurrentReconciles: conf.MaxConcurrentWorkers,
		Reconciler:              healthReconciler,
		RecoverPanic:            true,
	})
	if err != nil {
		return fmt.Errorf("unable to setup health reconciler: %w", err)
	}
	healthLogger := healthController.GetLogger()

	if conf.TargetCacheDisabled {
		// if the target cache is disable, we don't want to start additional informers
		healthReconciler.EnsureWatchForGVK = func(gvk schema.GroupVersionKind, obj client.Object) error {
			return nil
		}
	} else {
		watchedObjectGVKs := sync.Map{}
		healthReconciler.EnsureWatchForGVK = func(gvk schema.GroupVersionKind, obj client.Object) error {
			// check if we have already added watch for GVK
			// if not, store GVK in map
			if _, ok := watchedObjectGVKs.LoadOrStore(gvk, nil); ok {
				return nil
			}

			watchedObj := obj.DeepCopyObject().(client.Object)
			metadataOnly := false
			if !conf.TargetCluster.GetScheme().Recognizes(gvk) {
				// If we don't know the GVK, we definitely don't have a special health check for it.
				// I.e., we only care about whether the object is present or not.
				// Hence, we can start a metadata-only watch instead of watching the entire object, which saves bandwidth and
				// memory.
				metadataOnly = true
				metadataOnlyObj := &metav1.PartialObjectMetadata{}
				metadataOnlyObj.SetGroupVersionKind(gvk)
				watchedObj = metadataOnlyObj
			}

			healthLogger.Info("Adding new watch for GroupVersionKind", "groupVersionKind", gvk, "metadataOnly", metadataOnly)

			if err := healthController.Watch(
				&source.Kind{Type: watchedObj},
				handler.EnqueueRequestsFromMapFunc(mapToOriginManagedResource(healthLogger, conf.ClusterID)),
				HealthStatusChanged(healthLogger),
			); err != nil {
				return fmt.Errorf("error starting watch for GVK %s: %w", gvk.String(), err)
			}

			return nil
		}
	}

	if err := healthController.Watch(
		&source.Kind{Type: &resourcesv1alpha1.ManagedResource{}},
		enqueueCreateAndUpdate,
		append(healthControllerPredicates, &conf.ClassFilter)...,
	); err != nil {
		return fmt.Errorf("unable to watch ManagedResources: %w", err)
	}

	// setup reconciler for progressing condition
	log := mgr.GetLogger().WithName("controller").WithName(progressingReconcilerName)

	b := builder.ControllerManagedBy(mgr).Named(progressingReconcilerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: conf.MaxConcurrentWorkers,
			RecoverPanic:            true,
		}).
		For(&resourcesv1alpha1.ManagedResource{}, builder.WithPredicates(append(healthControllerPredicates, &conf.ClassFilter)...))

	if !conf.TargetCacheDisabled {
		// Watch relevant objects for Progressing condition in order to immediately update the condition as soon as there is
		// a change on managed resources.
		// If the target cache is disabled (e.g. for Shoots), we don't want to watch workload objects (Deployment, DaemonSet,
		// StatefulSet) because this would cache all of them in the entire cluster. This can potentially be a lot of objects
		// in Shoot clusters, because they are controlled by the end user. In this case, we rely on periodic syncs only.
		// If we want to have immediate updates for managed resources in Shoots in the future as well, we could consider
		// adding labels to managed resources and watch them explicitly.
		b.Watches(
			&source.Kind{Type: &appsv1.Deployment{}}, handler.EnqueueRequestsFromMapFunc(mapToOriginManagedResource(log, conf.ClusterID)),
			builder.WithPredicates(progressingStatusChanged),
		).Watches(
			&source.Kind{Type: &appsv1.StatefulSet{}}, handler.EnqueueRequestsFromMapFunc(mapToOriginManagedResource(log, conf.ClusterID)),
			builder.WithPredicates(progressingStatusChanged),
		).Watches(
			&source.Kind{Type: &appsv1.DaemonSet{}}, handler.EnqueueRequestsFromMapFunc(mapToOriginManagedResource(log, conf.ClusterID)),
			builder.WithPredicates(progressingStatusChanged),
		)
	}

	if err := b.Complete(&progressingReconciler{
		client:       mgr.GetClient(),
		targetClient: conf.TargetCluster.GetClient(),
		targetScheme: conf.TargetCluster.GetScheme(),
		classFilter:  &conf.ClassFilter,
		syncPeriod:   conf.SyncPeriod,
	}); err != nil {
		return fmt.Errorf("unable to setup progressing reconciler: %w", err)
	}

	return nil
}

// AddToManager adds the controller to a Manager using the default config.
func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, defaultControllerConfig)
}

// AddFlags adds the needed command line flags to the given FlagSet.
func (o *ControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.DurationVar(&o.syncPeriod, "health-sync-period", time.Minute, "duration how often the health of existing resources should be synced")
	fs.IntVar(&o.maxConcurrentWorkers, "health-max-concurrent-workers", 10, "number of worker threads for concurrent health reconciliation of resources")
}

// Complete completes the given command line flags and set the defaultControllerConfig accordingly.
func (o *ControllerOptions) Complete() error {
	defaultControllerConfig = ControllerConfig{
		MaxConcurrentWorkers: o.maxConcurrentWorkers,
		SyncPeriod:           o.syncPeriod,
	}
	return nil
}

// Completed returns the completed ControllerConfig.
func (o *ControllerOptions) Completed() *ControllerConfig {
	return &defaultControllerConfig
}

var enqueueCreateAndUpdate = &handler.Funcs{
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

var healthControllerPredicates = []predicate.Predicate{
	predicate.Or(
		managerpredicate.ClassChangedPredicate(),
		// start health checks immediately after MR has been reconciled
		managerpredicate.ConditionStatusChanged(resourcesv1alpha1.ResourcesApplied, managerpredicate.DefaultConditionChange),
		managerpredicate.NoLongerIgnored(),
	),
	managerpredicate.NotIgnored(),
}

func mapToOriginManagedResource(log logr.Logger, clusterID string) handler.MapFunc {
	return func(obj client.Object) []reconcile.Request {
		origin, ok := obj.GetAnnotations()[resourcesv1alpha1.OriginAnnotation]
		if !ok {
			return nil
		}

		originClusterID, key, err := resourceshelper.SplitOrigin(origin)
		if err != nil {
			log.Error(err, "Failed to parse origin of object", "object", obj, "origin", origin)
			return nil
		}

		if originClusterID != clusterID {
			// object isn't managed by this resource-manager instance
			return nil
		}

		return []reconcile.Request{{NamespacedName: key}}
	}
}

// HealthStatusChanged returns a predicate that filters for events that indicate a change in the object's health status.
func HealthStatusChanged(log logr.Logger) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetAnnotations()[resourcesv1alpha1.SkipHealthCheck] != "true"
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetAnnotations()[resourcesv1alpha1.SkipHealthCheck] != "true"
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld.GetResourceVersion() == e.ObjectNew.GetResourceVersion() {
				// periodic cache resync, enqueue
				return true
			}

			// ignore metadata-only update events
			if _, ok := e.ObjectOld.(*metav1.PartialObjectMetadata); ok {
				return false
			}
			if _, ok := e.ObjectNew.(*metav1.PartialObjectMetadata); ok {
				return false
			}

			var oldHealthy, newHealthy bool
			checked, oldErr := CheckHealth(e.ObjectOld)
			if !checked {
				if oldErr != nil {
					log.Error(oldErr, "Error determining health status of object", "object", e.ObjectOld)
				}
				return false
			}
			oldHealthy = oldErr != nil

			checked, newErr := CheckHealth(e.ObjectNew)
			if !checked {
				if newErr != nil {
					log.Error(newErr, "Error determining health status of object", "object", e.ObjectNew)
				}
				return false
			}
			newHealthy = newErr != nil

			return oldHealthy != newHealthy
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}

var progressingStatusChanged = predicate.Funcs{
	CreateFunc: func(_ event.CreateEvent) bool { return false },
	UpdateFunc: func(e event.UpdateEvent) bool {
		if e.ObjectOld.GetResourceVersion() == e.ObjectNew.GetResourceVersion() {
			// periodic cache resync, enqueue
			return true
		}

		oldProgressing, _ := CheckProgressing(e.ObjectOld)
		newProgressing, _ := CheckProgressing(e.ObjectNew)
		return oldProgressing != newProgressing
	},
	DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
	GenericFunc: func(_ event.GenericEvent) bool { return false },
}

func isIgnored(obj client.Object) bool {
	value, ok := obj.GetAnnotations()[resourcesv1alpha1.Ignore]
	if !ok {
		return false
	}
	truthy, _ := strconv.ParseBool(value)
	return truthy
}
