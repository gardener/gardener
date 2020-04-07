// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/util"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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
	o.ctx = util.ContextFromStopChannel(stopCh)
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
