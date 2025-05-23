// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacefinalizer

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
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
// It only finalizes them if their .metadata.finalizers[] do not contain an entry in Exceptions.
type Reconciler struct {
	Client             client.Client
	NamespaceFinalizer utilclient.Finalizer
	Exceptions         sets.Set[string]
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
		For(&corev1.Namespace{}, builder.WithPredicates(
			predicate.IsDeleting(),
			predicate.ForEventTypes(predicate.Update),
		)).
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

	if r.Exceptions.Intersection(sets.New[string](namespace.Finalizers...)).Len() > 0 {
		log.V(1).Info("Namespace has finalizers that are in the exception list, skipping finalization")
		return reconcile.Result{}, nil
	}

	log.V(1).Info("Finalizing Namespace")
	return reconcile.Result{}, r.NamespaceFinalizer.Finalize(ctx, r.Client, namespace)
}
