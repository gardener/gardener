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

package worker

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

type reconciler struct {
	logger   logr.Logger
	actuator Actuator

	ctx    context.Context
	client client.Client
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// Worker resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return extensionscontroller.OperationAnnotationWrapper(
		&extensionsv1alpha1.Worker{},
		&reconciler{
			logger:   log.Log.WithName(ControllerName),
			actuator: actuator,
		},
	)
}

func (r *reconciler) InjectFunc(f inject.Func) error {
	return f(r.actuator)
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

func (r *reconciler) InjectStopChannel(stopCh <-chan struct{}) error {
	r.ctx = util.ContextFromStopChannel(stopCh)
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	worker := &extensionsv1alpha1.Worker{}
	if err := r.client.Get(r.ctx, request.NamespacedName, worker); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cluster, err := extensionscontroller.GetCluster(r.ctx, r.client, worker.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Deletion flow
	if worker.DeletionTimestamp != nil {
		hasFinalizer, err := extensionscontroller.HasFinalizer(worker, FinalizerName)
		if err != nil {
			r.logger.Error(err, "Could not instantiate finalizer deletion")
			return reconcile.Result{}, err
		}
		if !hasFinalizer {
			r.logger.Info("Deleting worker causes a no-op as there is no finalizer.", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
			return reconcile.Result{}, nil
		}

		operationType := gardencorev1beta1helper.ComputeOperationType(worker.ObjectMeta, worker.Status.LastOperation)
		if err := r.updateStatusProcessing(r.ctx, worker, operationType, "Deleting the worker"); err != nil {
			return reconcile.Result{}, err
		}

		r.logger.Info("Starting the deletion of worker", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
		if err := r.actuator.Delete(r.ctx, worker, cluster); err != nil {
			msg := "Error deleting worker"
			utilruntime.HandleError(r.updateStatusError(r.ctx, extensionscontroller.ReconcileErrCauseOrErr(err), worker, operationType, msg))
			r.logger.Error(err, msg, "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
			return extensionscontroller.ReconcileErr(err)
		}

		msg := "Successfully deleted worker"
		r.logger.Info(msg, "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
		if err := r.updateStatusSuccess(r.ctx, worker, operationType, msg); err != nil {
			return reconcile.Result{}, err
		}

		r.logger.Info("Removing finalizer.", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
		if err := extensionscontroller.DeleteFinalizer(r.ctx, r.client, FinalizerName, worker); err != nil {
			r.logger.Error(err, "Error removing finalizer from Worker", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
			return reconcile.Result{}, err
		}

		return reconcile.Result{}, nil
	}

	// Reconcile flow
	if err := controller.EnsureFinalizer(r.ctx, r.client, FinalizerName, worker); err != nil {
		return reconcile.Result{}, err
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(worker.ObjectMeta, worker.Status.LastOperation)
	if err := r.updateStatusProcessing(r.ctx, worker, operationType, "Reconciling the worker"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Reconcile(r.ctx, worker, cluster); err != nil {
		msg := "Error reconciling worker"
		utilruntime.HandleError(r.updateStatusError(r.ctx, extensionscontroller.ReconcileErrCauseOrErr(err), worker, operationType, msg))
		r.logger.Error(err, msg, "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully reconciled worker"
	r.logger.Info(msg, "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
	if err := r.updateStatusSuccess(r.ctx, worker, operationType, msg); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) updateStatusProcessing(ctx context.Context, worker *extensionsv1alpha1.Worker, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, worker, func() error {
		worker.Status.LastOperation = extensionscontroller.LastOperation(lastOperationType, gardencorev1beta1.LastOperationStateProcessing, 1, description)
		return nil
	})
}

func (r *reconciler) updateStatusError(ctx context.Context, err error, worker *extensionsv1alpha1.Worker, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, worker, func() error {
		worker.Status.ObservedGeneration = worker.Generation
		worker.Status.LastOperation, worker.Status.LastError = extensionscontroller.ReconcileError(lastOperationType, gardencorev1beta1helper.FormatLastErrDescription(fmt.Errorf("%s: %v", description, err)), 50, gardencorev1beta1helper.ExtractErrorCodes(err)...)
		return nil
	})
}

func (r *reconciler) updateStatusSuccess(ctx context.Context, worker *extensionsv1alpha1.Worker, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, worker, func() error {
		worker.Status.ObservedGeneration = worker.Generation
		worker.Status.LastOperation, worker.Status.LastError = extensionscontroller.ReconcileSucceeded(lastOperationType, description)
		return nil
	})
}
