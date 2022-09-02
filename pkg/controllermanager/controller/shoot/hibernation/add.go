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

package hibernation

import (
	"reflect"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of this controller.
const ControllerName = "shoot-hibernation"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(ControllerName + "-controller")
	}

	// It's not possible to overwrite the event handler when using the controller builder. Hence, we have to build up
	// the controller manually.
	c, err := controller.New(
		ControllerName,
		mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
			RecoverPanic:            true,
		},
	)
	if err != nil {
		return err
	}

	return c.Watch(
		&source.Kind{Type: &gardencorev1beta1.Shoot{}},
		r.ShootEventHandler(c.GetLogger()),
		r.ShootPredicate(),
	)
}

// ShootPredicate returns the predicates for the core.gardener.cloud/v1beta1.Shoot watch.
func (r *Reconciler) ShootPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			shoot, ok := e.Object.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}
			return len(getShootHibernationSchedules(shoot.Spec.Hibernation)) > 0
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			shoot, ok := e.ObjectNew.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}

			oldShoot, ok := e.ObjectOld.(*gardencorev1beta1.Shoot)
			if !ok {
				return false
			}

			var (
				oldSchedules = getShootHibernationSchedules(oldShoot.Spec.Hibernation)
				newSchedules = getShootHibernationSchedules(shoot.Spec.Hibernation)
			)

			return !reflect.DeepEqual(oldSchedules, newSchedules) && len(newSchedules) > 0
		},
	}
}

// ShootEventHandler returns the event handler for the core.gardener.cloud/v1beta1.Shoot watch.
func (r *Reconciler) ShootEventHandler(log logr.Logger) handler.EventHandler {
	return handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			if e.Object == nil {
				return
			}
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      e.Object.GetName(),
				Namespace: e.Object.GetNamespace(),
			}})
		},
		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
			shoot, ok := e.ObjectNew.(*gardencorev1beta1.Shoot)
			if !ok {
				return
			}

			parsedSchedules, err := parseHibernationSchedules(getShootHibernationSchedules(shoot.Spec.Hibernation))
			if err != nil {
				log.Error(err, "Could not parse hibernation schedules for shoot", "shoot", client.ObjectKeyFromObject(shoot))
				return
			}

			q.AddAfter(reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      shoot.GetName(),
				Namespace: shoot.GetNamespace(),
			}}, nextHibernationTimeDuration(parsedSchedules, time.Now()))
		},
	}
}
