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

package token

import (
	"context"

	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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
			source.Func(func(_ context.Context, _ handler.EventHandler, q workqueue.RateLimitingInterface, _ ...predicate.Predicate) error {
				for _, config := range r.Config.SyncConfigs {
					q.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: config.SecretName, Namespace: metav1.NamespaceSystem}})
				}
				return nil
			}),
			&handler.EnqueueRequestForObject{},
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: len(r.Config.SyncConfigs)}).
		Complete(r)
}
