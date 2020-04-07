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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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

	operationType := gardencorev1beta1helper.ComputeOperationType(worker.ObjectMeta, worker.Status.LastOperation)

	switch {
	case isWorkerMigrated(worker):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(worker, cluster)
	case worker.DeletionTimestamp != nil:
		return r.delete(worker, cluster)
	case worker.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore:
		return r.restore(worker, cluster, operationType)
	default:
		return r.reconcile(worker, cluster, operationType)
	}
}

func (r *reconciler) updateStatusProcessing(ctx context.Context, worker *extensionsv1alpha1.Worker, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	r.logger.Info(description, "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, worker, func() error {
		worker.Status.LastOperation = extensionscontroller.LastOperation(lastOperationType, gardencorev1beta1.LastOperationStateProcessing, 1, description)
		return nil
	})
}

func (r *reconciler) updateStatusError(ctx context.Context, err error, worker *extensionsv1alpha1.Worker, lastOperationType gardencorev1beta1.LastOperationType, description string) {
	r.logger.Error(err, description, "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
	updateErr := extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, worker, func() error {
		worker.Status.ObservedGeneration = worker.Generation
		worker.Status.LastOperation, worker.Status.LastError = extensionscontroller.ReconcileError(lastOperationType, gardencorev1beta1helper.FormatLastErrDescription(fmt.Errorf("%s: %v", description, extensionscontroller.ReconcileErrCauseOrErr(err))), 50, gardencorev1beta1helper.ExtractErrorCodes(err)...)
		return nil
	})
	utilruntime.HandleError(updateErr)
}

func (r *reconciler) updateStatusSuccess(ctx context.Context, worker *extensionsv1alpha1.Worker, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	r.logger.Info(description, "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, worker, func() error {
		worker.Status.ObservedGeneration = worker.Generation
		worker.Status.LastOperation, worker.Status.LastError = extensionscontroller.ReconcileSucceeded(lastOperationType, description)
		return nil
	})
}

func (r *reconciler) removeFinalizerFromWorker(worker *extensionsv1alpha1.Worker) error {
	r.logger.Info("Removing finalizer.", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
	if err := extensionscontroller.DeleteFinalizer(r.ctx, r.client, FinalizerName, worker); err != nil {
		r.logger.Error(err, "Error removing finalizer from Worker", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
		return err
	}
	return nil
}

func (r *reconciler) migrate(worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := r.updateStatusProcessing(r.ctx, worker, gardencorev1beta1.LastOperationTypeMigrate, "Starting Migration of the worker"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Migrate(r.ctx, worker, cluster); err != nil {
		r.updateStatusError(r.ctx, extensionscontroller.ReconcileErrCauseOrErr(err), worker, gardencorev1beta1.LastOperationTypeMigrate, "Error migrating worker")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(r.ctx, worker, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrate worker"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.removeFinalizerFromWorker(worker); err != nil {
		return reconcile.Result{}, err
	}

	// remove operation annotation 'migrate'
	if err := removeAnnotation(r.ctx, r.client, worker, v1beta1constants.GardenerOperation); err != nil {
		r.logger.Error(err, "Error removing annotation from Worker", "annotation", fmt.Sprintf("%s/%s", v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationMigrate), "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	hasFinalizer, err := extensionscontroller.HasFinalizer(worker, FinalizerName)
	if err != nil {
		r.logger.Error(err, "Could not instantiate finalizer deletion")
		return reconcile.Result{}, err
	}
	if !hasFinalizer {
		r.logger.Info("Deleting worker causes a no-op as there is no finalizer.", "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
		return reconcile.Result{}, nil
	}

	if err := r.updateStatusProcessing(r.ctx, worker, gardencorev1beta1.LastOperationTypeDelete, "Deleting the worker"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Delete(r.ctx, worker, cluster); err != nil {
		r.updateStatusError(r.ctx, extensionscontroller.ReconcileErrCauseOrErr(err), worker, gardencorev1beta1.LastOperationTypeDelete, "Error deleting worker")

		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(r.ctx, worker, gardencorev1beta1.LastOperationTypeDelete, "Successfully deleted worker"); err != nil {
		return reconcile.Result{}, err
	}

	err = r.removeFinalizerFromWorker(worker)
	return reconcile.Result{}, err
}

func (r *reconciler) reconcile(worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := controller.EnsureFinalizer(r.ctx, r.client, FinalizerName, worker); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.updateStatusProcessing(r.ctx, worker, operationType, "Reconciling the worker"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Reconcile(r.ctx, worker, cluster); err != nil {
		r.updateStatusError(r.ctx, err, worker, operationType, "Error reconciling worker")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(r.ctx, worker, operationType, "Successfully reconciled worker"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := r.updateStatusProcessing(r.ctx, worker, operationType, "Restoring the worker"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Restore(r.ctx, worker, cluster); err != nil {
		r.updateStatusError(r.ctx, extensionscontroller.ReconcileErrCauseOrErr(err), worker, operationType, "Error restoring worker")
		return extensionscontroller.ReconcileErr(err)
	}

	// remove operation annotation 'restore'
	if err := removeAnnotation(r.ctx, r.client, worker, v1beta1constants.GardenerOperation); err != nil {
		r.logger.Error(err, "Error removing annotation from Worker", "annotation", fmt.Sprintf("%s/%s", v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRestore), "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
		return reconcile.Result{}, err
	}

	// requeue to triger reconciliation
	return reconcile.Result{Requeue: true}, nil
}

func removeAnnotation(ctx context.Context, c client.Client, worker *extensionsv1alpha1.Worker, annotation string) error {
	withOpAnnotation := worker.DeepCopyObject()
	delete(worker.Annotations, annotation)
	return c.Patch(ctx, worker, client.MergeFrom(withOpAnnotation))
}

func isWorkerMigrated(worker *extensionsv1alpha1.Worker) bool {
	return worker.Status.LastOperation != nil &&
		worker.Status.LastOperation.GetType() == gardencorev1beta1.LastOperationTypeMigrate &&
		worker.Status.LastOperation.GetState() == gardencorev1beta1.LastOperationStateSucceeded
}
