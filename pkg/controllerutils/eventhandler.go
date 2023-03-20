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

package controllerutils

import (
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func reconcileRequest(obj client.Object) reconcile.Request {
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
			q.AddAfter(reconcileRequest(evt.Object), getDuration(evt.Object, clock))
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

func getDuration(o client.Object, clock clock.Clock) time.Duration {
	switch obj := o.(type) {
	case *gardencorev1beta1.BackupBucket:
		return ReconcileOncePer24hDuration(clock, obj.ObjectMeta, obj.Status.ObservedGeneration, obj.Status.LastOperation)
	case *gardencorev1beta1.BackupEntry:
		return ReconcileOncePer24hDuration(clock, obj.ObjectMeta, obj.Status.ObservedGeneration, obj.Status.LastOperation)
	}
	return 0
}
