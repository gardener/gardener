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

package reconciler

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
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
func OperationAnnotationWrapper(newObjFunc func() client.Object, reconciler reconcile.Reconciler) reconcile.Reconciler {
	return &operationAnnotationWrapper{
		newObjFunc: newObjFunc,
		Reconciler: reconciler,
	}
}

func (o *operationAnnotationWrapper) InjectClient(client client.Client) error {
	o.client = client
	return nil
}

func (o *operationAnnotationWrapper) InjectFunc(f inject.Func) error {
	return f(o.Reconciler)
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
