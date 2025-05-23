// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operationannotation

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// Reconciler is a reconciler that removes operation annotations from objects.
// This is useful for integration testing against an envtest control plane, which doesn't run the responsible
// controller. Hence, if the tested controller must wait for the annotation to be removed, it will be stuck forever.
type Reconciler struct {
	Client    client.Client
	ForObject func() client.Object
}

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return builder.ControllerManagedBy(mgr).
		Named("operationannotation").
		For(r.ForObject(), builder.WithPredicates(r.hasOperationAnnotation())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
		}).
		Complete(r)
}

// Reconcile removes the operation annotation from the object.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	obj := r.ForObject()
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	log.V(1).Info("Removing operation annotation")

	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	delete(obj.GetAnnotations(), v1beta1constants.GardenerOperation)
	return reconcile.Result{}, r.Client.Patch(ctx, obj, patch)
}

func (r *Reconciler) hasOperationAnnotation() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetAnnotations()[v1beta1constants.GardenerOperation] != ""
	})
}
