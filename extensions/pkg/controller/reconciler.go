// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	contextutil "github.com/gardener/gardener/pkg/utils/context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

type operationAnnotationWrapper struct {
	reconcile.Reconciler
	client     client.Client
	ctx        context.Context
	objectType runtime.Object
}

// OperationAnnotationWrapper is a wrapper for an reconciler that
// removes the Gardener operation annotation before `Reconcile` is called.
//
// This is useful in conjunction with the HasOperationAnnotationPredicate.
func OperationAnnotationWrapper(objectType runtime.Object, reconciler reconcile.Reconciler) reconcile.Reconciler {
	return &operationAnnotationWrapper{
		objectType: objectType,
		Reconciler: reconciler,
	}
}

// InjectClient implements inject.Client.
func (o *operationAnnotationWrapper) InjectClient(client client.Client) error {
	o.client = client
	return nil
}

// InjectClient implements inject.Func.
func (o *operationAnnotationWrapper) InjectFunc(f inject.Func) error {
	return f(o.Reconciler)
}

// InjectStopChannel is an implementation for getting the respective stop channel managed by the controller-runtime.
func (o *operationAnnotationWrapper) InjectStopChannel(stopCh <-chan struct{}) error {
	o.ctx = contextutil.FromStopChannel(stopCh)
	return nil
}

// Reconcile removes the Gardener operation annotation if available and calls the inner `Reconcile`.
func (o *operationAnnotationWrapper) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	obj := o.objectType.DeepCopyObject()
	if err := o.client.Get(o.ctx, request.NamespacedName, obj); client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, err
	}

	acc, err := meta.Accessor(obj)
	if err != nil {
		return reconcile.Result{}, err
	}

	annotations := acc.GetAnnotations()
	if annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationWaitForState {
		return reconcile.Result{}, nil
	}

	if annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationReconcile {
		withOpAnnotation := obj.DeepCopyObject()
		delete(annotations, v1beta1constants.GardenerOperation)
		acc.SetAnnotations(annotations)
		if err := o.client.Patch(o.ctx, obj, client.MergeFrom(withOpAnnotation)); err != nil {
			return reconcile.Result{}, err
		}
	}
	return o.Reconciler.Reconcile(request)
}
