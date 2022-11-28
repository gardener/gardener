// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
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
	if r.ReconciliationDueTracker == nil {
		r.ReconciliationDueTracker = newReconciliationDueTracker()
	}

	// It's not possible to overwrite the event handler when using the controller builder. Hence, we have to build up
	// the controller manually.
	c, err := controller.New(
		ControllerName,
		mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.Controllers.Shoot.ConcurrentSyncs, 0),
			RecoverPanic:            true,
		},
	)
	if err != nil {
		return err
	}

	return c.Watch(
		source.NewKindWithCache(&gardencorev1beta1.Shoot{}, gardenCluster.GetCache()),
		r.EventHandler(),
		predicateutils.SeedNamePredicate(r.Config.SeedConfig.Name, gutil.GetShootSeedNames),
		&predicate.GenerationChangedPredicate{},
	)
}

// EventHandler returns an event handler.
func (r *Reconciler) EventHandler() handler.EventHandler {
	return &handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      e.Object.GetName(),
				Namespace: e.Object.GetNamespace(),
			}})
		},
		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
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
