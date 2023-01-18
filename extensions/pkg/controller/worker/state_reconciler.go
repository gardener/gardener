// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package worker

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
)

type stateReconciler struct {
	actuator StateActuator

	client client.Client
}

// NewStateReconciler creates a new reconcile.Reconciler that reconciles
// Worker's State resources of Gardener's `extensions.gardener.cloud` API group.
func NewStateReconciler(actuator StateActuator) reconcile.Reconciler {
	return &stateReconciler{
		actuator: actuator,
	}
}

func (r *stateReconciler) InjectFunc(f inject.Func) error {
	return f(r.actuator)
}

func (r *stateReconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

func (r *stateReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	worker := &extensionsv1alpha1.Worker{}
	if err := r.client.Get(ctx, request.NamespacedName, worker); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// Deletion flow
	if worker.DeletionTimestamp != nil {
		// Nothing to do
		return reconcile.Result{}, nil
	}

	// Reconcile flow
	operationType := v1beta1helper.ComputeOperationType(worker.ObjectMeta, worker.Status.LastOperation)
	if operationType != gardencorev1beta1.LastOperationTypeReconcile {
		return reconcile.Result{Requeue: true}, nil
	} else if isWorkerMigrated(worker) {
		return reconcile.Result{}, nil
	}

	if err := r.actuator.Reconcile(ctx, log, worker); err != nil {
		return reconcilerutils.ReconcileErr(fmt.Errorf("error updating Worker state: %w", err))
	}

	log.Info("Successfully updated Worker state")
	return reconcile.Result{}, nil
}
