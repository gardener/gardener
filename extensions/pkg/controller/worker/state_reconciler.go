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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	errorutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

type stateReconciler struct {
	logger   logr.Logger
	actuator StateActuator

	client client.Client
}

// NewStateReconciler creates a new reconcile.Reconciler that reconciles
// Worker's State resources of Gardener's `extensions.gardener.cloud` API group.
func NewStateReconciler(mgr manager.Manager, actuator StateActuator) reconcile.Reconciler {
	return &stateReconciler{
		logger:   log.Log.WithName(StateUpdatingControllerName),
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
	worker := &extensionsv1alpha1.Worker{}
	if err := r.client.Get(ctx, request.NamespacedName, worker); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	logger := r.logger.WithValues("worker", client.ObjectKeyFromObject(worker))

	// Deletion flow
	if worker.DeletionTimestamp != nil {
		// Nothing to do
		return reconcile.Result{}, nil
	}

	// Reconcile flow
	operationType := gardencorev1beta1helper.ComputeOperationType(worker.ObjectMeta, worker.Status.LastOperation)
	if operationType != gardencorev1beta1.LastOperationTypeReconcile {
		return reconcile.Result{Requeue: true}, nil
	} else if isWorkerMigrated(worker) {
		// Nothing to do
		return reconcile.Result{}, nil
	}

	if err := r.actuator.Reconcile(ctx, worker); err != nil {
		return errorutils.ReconcileErr(fmt.Errorf("error updating worker state: %w", err))
	}

	logger.Info("Successfully updated worker state")

	return reconcile.Result{}, nil
}
