// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of this controller.
const ControllerName = "token"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, channel <-chan event.TypedGenericEvent[*corev1.Secret]) error {
	if r.APIReader == nil {
		r.APIReader = mgr.GetAPIReader()
	}

	if r.FS.Fs == nil {
		r.FS = afero.Afero{Fs: afero.NewOsFs()}
	}

	r.secretNameToPath = make(map[string]string, len(r.Config.SyncConfigs))
	for _, config := range r.Config.SyncConfigs {
		r.secretNameToPath[config.SecretName] = config.Path
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WatchesRawSource(
			source.Func(func(_ context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
				for _, config := range r.Config.SyncConfigs {
					q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: config.SecretName, Namespace: metav1.NamespaceSystem}})
				}
				return nil
			}),
		).
		WatchesRawSource(
			source.TypedChannel(channel, r.EventHandler()),
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: len(r.Config.SyncConfigs)}).
		Complete(r)
}

// EventHandler returns a handler for corev1.Secret events.
func (r *Reconciler) EventHandler() handler.TypedEventHandler[*corev1.Secret, reconcile.Request] {
	return &handler.TypedFuncs[*corev1.Secret, reconcile.Request]{
		GenericFunc: func(_ context.Context, e event.TypedGenericEvent[*corev1.Secret], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			if e.Object != nil {
				q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: e.Object.GetName(), Namespace: e.Object.GetNamespace()}})
			}
		},
	}
}
