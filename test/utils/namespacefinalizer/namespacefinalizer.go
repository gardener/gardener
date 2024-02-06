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

package namespacefinalizer

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/controllerutils/predicate"
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"
)

// Reconciler is a reconciler that finalizes namespaces once they are marked for deletion.
// This is useful for integration testing against an envtest control plane, which doesn't run the namespace controller.
// Hence, if the tested controller must wait for a namespace to be deleted, it will be stuck forever.
// This reconciler finalizes namespaces without deleting their contents, so use with care.
type Reconciler struct {
	Client             client.Client
	NamespaceFinalizer utilclient.Finalizer
}

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.NamespaceFinalizer == nil {
		r.NamespaceFinalizer = utilclient.NewNamespaceFinalizer()
	}

	return builder.ControllerManagedBy(mgr).
		Named("namespacefinalizer").
		For(&corev1.Namespace{}, builder.WithPredicates(predicate.IsDeleting())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
			RecoverPanic:            ptr.To(true),
		}).
		Complete(r)
}

// Reconcile finalizes namespaces as soon as they are marked for deletion.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	namespace := &corev1.Namespace{}
	if err := r.Client.Get(ctx, req.NamespacedName, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if namespace.DeletionTimestamp == nil {
		return reconcile.Result{}, nil
	}

	log.V(1).Info("Finalizing Namespace")
	return reconcile.Result{}, r.NamespaceFinalizer.Finalize(ctx, r.Client, namespace)
}
