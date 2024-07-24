// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"

	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of this controller.
const ControllerName = "token"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
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
			source.Func(func(_ context.Context, q workqueue.RateLimitingInterface) error {
				for _, config := range r.Config.SyncConfigs {
					q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: config.SecretName, Namespace: metav1.NamespaceSystem}})
				}
				return nil
			}),
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: len(r.Config.SyncConfigs)}).
		Complete(r)
}
