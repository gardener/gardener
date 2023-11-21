// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package operatingsystemconfig

import (
	"bytes"
	"context"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	"github.com/gardener/gardener/pkg/nodeagent/registry"
	"github.com/gardener/gardener/pkg/utils"
)

// ControllerName is the name of this controller.
const ControllerName = "operatingsystemconfig"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(ControllerName)
	}
	if r.DBus == nil {
		r.DBus = dbus.New()
	}
	if r.FS.Fs == nil {
		r.FS = afero.Afero{Fs: afero.NewOsFs()}
	}
	if r.Extractor == nil {
		r.Extractor = registry.NewExtractor()
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WatchesRawSource(
			source.Kind(mgr.GetCache(), &corev1.Secret{}),
			r.EnqueueWithJitterDelay(mgr.GetLogger().WithValues("controller", ControllerName).WithName("jitterEventHandler")),
			builder.WithPredicates(
				predicate.NewPredicateFuncs(func(obj client.Object) bool {
					return obj.GetNamespace() == metav1.NamespaceSystem && obj.GetName() == r.Config.SecretName
				}),
				r.SecretPredicate(),
				predicateutils.ForEventTypes(predicateutils.Create, predicateutils.Update),
			),
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// SecretPredicate returns the predicate for Secret events.
func (r *Reconciler) SecretPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldSecret, ok := e.ObjectOld.(*corev1.Secret)
			if !ok {
				return false
			}

			newSecret, ok := e.ObjectNew.(*corev1.Secret)
			if !ok {
				return false
			}

			return !bytes.Equal(oldSecret.Data[nodeagentv1alpha1.DataKeyOperatingSystemConfig], newSecret.Data[nodeagentv1alpha1.DataKeyOperatingSystemConfig])
		},
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

func reconcileRequest(obj client.Object) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}}
}

// RandomDurationWithMetaDuration is an alias for `utils.RandomDurationWithMetaDuration`. Exposed for unit tests.
var RandomDurationWithMetaDuration = utils.RandomDurationWithMetaDuration

// EnqueueWithJitterDelay returns handler.Funcs which enqueues the object with a random jitter duration for 'update'
// events. 'Create' events are enqueued immediately.
func (r *Reconciler) EnqueueWithJitterDelay(log logr.Logger) handler.EventHandler {
	return &handler.Funcs{
		CreateFunc: func(_ context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
			if evt.Object == nil {
				return
			}
			q.Add(reconcileRequest(evt.Object))
		},

		UpdateFunc: func(_ context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
			oldSecret, ok := evt.ObjectOld.(*corev1.Secret)
			if !ok {
				return
			}
			newSecret, ok := evt.ObjectNew.(*corev1.Secret)
			if !ok {
				return
			}

			if !bytes.Equal(oldSecret.Data[nodeagentv1alpha1.DataKeyOperatingSystemConfig], newSecret.Data[nodeagentv1alpha1.DataKeyOperatingSystemConfig]) {
				duration := RandomDurationWithMetaDuration(r.Config.SyncJitterPeriod)
				log.Info("Enqueued secret with operating system config with a jitter period", "duration", duration)
				q.AddAfter(reconcileRequest(evt.ObjectNew), duration)
			}
		},
	}
}
