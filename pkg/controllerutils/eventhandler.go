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

package controllerutils

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/utils"
)

var reconcileRequest = func(obj client.Object) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}}
}

// EnqueueCreateEventsOncePer24hDuration returns handler.Funcs which enqueues the object for Create events only once per 24h.
// All other events are normally enqueued.
func EnqueueCreateEventsOncePer24hDuration(clock clock.Clock) handler.Funcs {
	return handler.Funcs{
		CreateFunc: func(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
			if evt.Object == nil {
				return
			}
			q.AddAfter(reconcileRequest(evt.Object), getDuration(evt.Object, clock, nil))
		},
		UpdateFunc: func(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
			if evt.ObjectNew == nil {
				return
			}
			q.Add(reconcileRequest(evt.ObjectNew))
		},
		DeleteFunc: func(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
			if evt.Object == nil {
				return
			}
			q.Add(reconcileRequest(evt.Object))
		},
	}
}

// EnqueueWithJitterDelay returns handler.Funcs which enqueues the object with a random Jitter duration when the JitterUpdate
// is enabled in ManagedSeed controller configuration.
// All other events are normally enqueued.
func EnqueueWithJitterDelay(cfg config.ManagedSeedControllerConfiguration) handler.Funcs {
	return handler.Funcs{
		CreateFunc: func(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
			managedSeed, ok := evt.Object.(*seedmanagementv1alpha1.ManagedSeed)
			if !ok {
				return
			}

			generationChanged := managedSeed.Generation != managedSeed.Status.ObservedGeneration

			// Managed seed with deletion timestamp and newly created managed seed will be enqueued immediately.
			// Generation is 1 for newly created objects.
			if managedSeed.DeletionTimestamp != nil || managedSeed.Generation == 1 {
				q.Add(reconcileRequest(evt.Object))
				return
			}

			if generationChanged {
				if *cfg.JitterUpdates {
					q.AddAfter(reconcileRequest(evt.Object), getDuration(evt.Object, nil, cfg.SyncJitterPeriod))
				} else {
					q.Add(reconcileRequest(evt.Object))
				}
			} else {
				// Spread reconciliation of managed seeds (including gardenlet updates/rollouts) across the configured sync jitter
				// period to avoid overloading the gardener-apiserver if all gardenlets in all managed seeds are (re)starting
				// roughly at the same time
				q.AddAfter(reconcileRequest(evt.Object), getDuration(evt.Object, nil, cfg.SyncJitterPeriod))
			}
		},
		UpdateFunc: func(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
			managedSeed, ok := evt.ObjectNew.(*seedmanagementv1alpha1.ManagedSeed)
			if !ok {
				return
			}

			if managedSeed.Generation == managedSeed.Status.ObservedGeneration {
				return
			}

			// Managed seed with deletion timestamp and newly created managed seed will be enqueued immediately.
			// Generation is 1 for newly created objects.
			if managedSeed.DeletionTimestamp != nil || managedSeed.Generation == 1 {
				q.Add(reconcileRequest(evt.ObjectNew))
				return
			}

			if *cfg.JitterUpdates {
				q.AddAfter(reconcileRequest(evt.ObjectNew), getDuration(evt.ObjectNew, nil, cfg.SyncJitterPeriod))
			} else {
				q.Add(reconcileRequest(evt.ObjectNew))
			}
		},
		DeleteFunc: func(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
			if evt.Object == nil {
				return
			}
			q.Add(reconcileRequest(evt.Object))
		},
	}
}

func getDuration(obj client.Object, clock clock.Clock, syncJitterPeriod *metav1.Duration) time.Duration {
	switch obj := obj.(type) {
	case *gardencorev1beta1.BackupBucket:
		return ReconcileOncePer24hDuration(clock, obj.ObjectMeta, obj.Status.ObservedGeneration, obj.Status.LastOperation)
	case *gardencorev1beta1.BackupEntry:
		return ReconcileOncePer24hDuration(clock, obj.ObjectMeta, obj.Status.ObservedGeneration, obj.Status.LastOperation)
	case *seedmanagementv1alpha1.ManagedSeed:
		return utils.RandomDurationWithMetaDuration(syncJitterPeriod)
	}
	return 0
}
