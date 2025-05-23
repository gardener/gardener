// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reconciler

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

type operationAnnotationWrapper struct {
	reconcile.Reconciler
	client     client.Client
	newObjFunc func() client.Object
}

// OperationAnnotationWrapper is a wrapper for an reconciler that
// removes the Gardener operation annotation before `Reconcile` is called.
//
// This is useful in conjunction with the HasOperationAnnotation predicate.
func OperationAnnotationWrapper(mgr manager.Manager, newObjFunc func() client.Object, reconciler reconcile.Reconciler) reconcile.Reconciler {
	return &operationAnnotationWrapper{
		client:     mgr.GetClient(),
		newObjFunc: newObjFunc,
		Reconciler: reconciler,
	}
}

func (o *operationAnnotationWrapper) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	obj := o.newObjFunc()
	if err := o.client.Get(ctx, request.NamespacedName, obj); client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, err
	}

	annotations := obj.GetAnnotations()
	if annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationWaitForState {
		return reconcile.Result{}, nil
	}

	if annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationReconcile {
		withOpAnnotation := obj.DeepCopyObject().(client.Object)
		delete(annotations, v1beta1constants.GardenerOperation)
		obj.SetAnnotations(annotations)
		if err := o.client.Patch(ctx, obj, client.MergeFrom(withOpAnnotation)); err != nil {
			return reconcile.Result{}, err
		}
	}

	return o.Reconciler.Reconcile(ctx, request)
}
