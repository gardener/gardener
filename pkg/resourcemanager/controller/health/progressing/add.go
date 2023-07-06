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

package progressing

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/health/utils"
	resourcemanagerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

// ControllerName is the name of the controller.
const ControllerName = "health-progressing"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager, sourceCluster, targetCluster cluster.Cluster, targetCacheDisabled bool, clusterID string) error {
	if r.SourceClient == nil {
		r.SourceClient = sourceCluster.GetClient()
	}
	if r.TargetClient == nil {
		r.TargetClient = targetCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	c, err := controller.New(
		ControllerName,
		mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
		},
	)
	if err != nil {
		return err
	}

	b := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&resourcesv1alpha1.ManagedResource{}, builder.WithPredicates(
			predicate.Or(
				resourcemanagerpredicate.ClassChangedPredicate(),
				// start health checks immediately after MR has been reconciled
				resourcemanagerpredicate.ConditionStatusChanged(resourcesv1alpha1.ResourcesApplied, resourcemanagerpredicate.DefaultConditionChange),
				resourcemanagerpredicate.NoLongerIgnored(),
			),
			resourcemanagerpredicate.NotIgnored(),
			r.ClassFilter,
		)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
		})

	if !targetCacheDisabled {
		// Watch relevant objects for Progressing condition in order to immediately update the condition as soon as there is
		// a change on managed resources.
		// If the target cache is disabled (e.g. for Shoots), we don't want to watch workload objects (Deployment, DaemonSet,
		// StatefulSet) because this would cache all of them in the entire cluster. This can potentially be a lot of objects
		// in Shoot clusters, because they are controlled by the end user. In this case, we rely on periodic syncs only.
		// If we want to have immediate updates for managed resources in Shoots in the future as well, we could consider
		// adding labels to managed resources and watch them explicitly.
		b = b.Watches(
			source.NewKindWithCache(&appsv1.Deployment{}, targetCluster.GetCache()),
			mapper.EnqueueRequestsFrom(ctx, mgr, utils.MapToOriginManagedResource(clusterID), mapper.UpdateWithNew, c.GetLogger()),
			builder.WithPredicates(r.ProgressingStatusChanged()),
		).Watches(
			source.NewKindWithCache(&appsv1.StatefulSet{}, targetCluster.GetCache()),
			mapper.EnqueueRequestsFrom(ctx, mgr, utils.MapToOriginManagedResource(clusterID), mapper.UpdateWithNew, c.GetLogger()),
			builder.WithPredicates(r.ProgressingStatusChanged()),
		).Watches(
			source.NewKindWithCache(&appsv1.DaemonSet{}, targetCluster.GetCache()),
			mapper.EnqueueRequestsFrom(ctx, mgr, utils.MapToOriginManagedResource(clusterID), mapper.UpdateWithNew, c.GetLogger()),
			builder.WithPredicates(r.ProgressingStatusChanged()),
		)
	}

	return b.Complete(r)
}

// ProgressingStatusChanged returns a predicate that filters for events that indicate a change in the object's
// progressing status.
func (r *Reconciler) ProgressingStatusChanged() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld.GetResourceVersion() == e.ObjectNew.GetResourceVersion() {
				// periodic cache resync, enqueue
				return true
			}

			oldProgressing, _ := checkProgressing(e.ObjectOld)
			newProgressing, _ := checkProgressing(e.ObjectNew)

			return oldProgressing != newProgressing
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}
