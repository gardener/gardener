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

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// EnqueueCreateEventsOncePer24hDuration enqueues the object for Create events only once per 24h.
// All other events are normally enqueued.
var EnqueueCreateEventsOncePer24hDuration = handler.Funcs{
	CreateFunc: func(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
		if evt.Object == nil {
			return
		}
		q.AddAfter(reconcile.Request{NamespacedName: types.NamespacedName{
			Name:      evt.Object.GetName(),
			Namespace: evt.Object.GetNamespace(),
		}}, getDuration(evt.Object))
	},
	UpdateFunc: func(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
		switch {
		case evt.ObjectNew != nil:
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      evt.ObjectNew.GetName(),
				Namespace: evt.ObjectNew.GetNamespace(),
			}})
		case evt.ObjectOld != nil:
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      evt.ObjectOld.GetName(),
				Namespace: evt.ObjectOld.GetNamespace(),
			}})
		}
	},
	DeleteFunc: func(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
		if evt.Object == nil {
			return
		}
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Name:      evt.Object.GetName(),
			Namespace: evt.Object.GetNamespace(),
		}})
	},
	GenericFunc: func(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
		if evt.Object == nil {
			return
		}
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Name:      evt.Object.GetName(),
			Namespace: evt.Object.GetNamespace(),
		}})
	},
}

func getDuration(obj client.Object) time.Duration {
	switch obj := obj.(type) {
	case *gardencorev1beta1.BackupBucket:
		return ReconcileOncePer24hDuration(obj.ObjectMeta, obj.Status.ObservedGeneration, obj.Status.LastOperation)
	case *gardencorev1beta1.BackupEntry:
		return ReconcileOncePer24hDuration(obj.ObjectMeta, obj.Status.ObservedGeneration, obj.Status.LastOperation)
	}
	return 0
}
