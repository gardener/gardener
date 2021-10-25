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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type reconciler struct {
	logger          logr.Logger
	actuator        Actuator
	watchdogManager common.WatchdogManager

	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// Worker resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(actuator Actuator, watchdogManager common.WatchdogManager) reconcile.Reconciler {
	logger := log.Log.WithName(ControllerName)

	return reconcilerutils.OperationAnnotationWrapper(
		func() client.Object { return &extensionsv1alpha1.Worker{} },
		&reconciler{
			logger:          logger,
			actuator:        actuator,
			watchdogManager: watchdogManager,
			statusUpdater:   extensionscontroller.NewStatusUpdater(logger),
		},
	)
}

func (r *reconciler) InjectFunc(f inject.Func) error {
	return f(r.actuator)
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	r.statusUpdater.InjectClient(client)
	return nil
}

func (r *reconciler) InjectAPIReader(reader client.Reader) error {
	r.reader = reader
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	worker := &extensionsv1alpha1.Worker{}
	if err := r.client.Get(ctx, request.NamespacedName, worker); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, worker.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	logger := r.logger.WithValues("worker", client.ObjectKeyFromObject(worker))
	if extensionscontroller.IsFailed(cluster) {
		logger.Info("Skipping of the reconciliation of worker of failed shoot", "worker", kutil.ObjectName(worker))
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(worker.ObjectMeta, worker.Status.LastOperation)

	if cluster.Shoot != nil && operationType != gardencorev1beta1.LastOperationTypeMigrate {
		key := "worker:" + kutil.ObjectName(worker)
		ok, watchdogCtx, cleanup, err := r.watchdogManager.GetResultAndContext(ctx, r.client, worker.Namespace, cluster.Shoot.Name, key)
		if err != nil {
			return reconcile.Result{}, err
		} else if !ok {
			return reconcile.Result{}, fmt.Errorf("this seed is not the owner of shoot %s", kutil.ObjectName(cluster.Shoot))
		}
		ctx = watchdogCtx
		if cleanup != nil {
			defer cleanup()
		}
	}

	switch {
	case extensionscontroller.ShouldSkipOperation(operationType, worker):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, logger.WithValues("operation", "migrate"), worker, cluster)
	case worker.DeletionTimestamp != nil:
		return r.delete(ctx, logger.WithValues("operation", "delete"), worker, cluster)
	case operationType == gardencorev1beta1.LastOperationTypeRestore:
		return r.restore(ctx, logger.WithValues("operation", "restore"), worker, cluster)
	default:
		return r.reconcile(ctx, logger.WithValues("operation", "reconcile"), worker, cluster, operationType)
	}
}

func (r *reconciler) removeFinalizerFromWorker(ctx context.Context, logger logr.Logger, worker *extensionsv1alpha1.Worker) error {
	logger.Info("Removing all finalizers", "worker", kutil.ObjectName(worker))
	if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, worker, FinalizerName); err != nil {
		return fmt.Errorf("error removing finalizer from worker: %+v", err)
	}
	return nil
}

func (r *reconciler) removeAnnotation(ctx context.Context, logger logr.Logger, worker *extensionsv1alpha1.Worker) error {
	return extensionscontroller.RemoveAnnotation(ctx, r.client, worker, v1beta1constants.GardenerOperation)
}

func (r *reconciler) migrate(ctx context.Context, logger logr.Logger, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := r.statusUpdater.Processing(ctx, worker, gardencorev1beta1.LastOperationTypeMigrate, "Starting Migration of the worker"); err != nil {
		return reconcile.Result{}, err
	}

	logger.Info("Starting the migration of worker", "worker", kutil.ObjectName(worker))
	if err := r.actuator.Migrate(ctx, worker, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, worker, err, gardencorev1beta1.LastOperationTypeMigrate, "Error migrating worker")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, worker, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrate worker"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.removeFinalizerFromWorker(ctx, logger, worker); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.removeAnnotation(ctx, logger, worker); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from worker: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, logger logr.Logger, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(worker, FinalizerName) {
		logger.Info("Deleting worker causes a no-op as there is no finalizer", "worker", kutil.ObjectName(worker))
		return reconcile.Result{}, nil
	}

	if err := r.statusUpdater.Processing(ctx, worker, gardencorev1beta1.LastOperationTypeDelete, "Deleting the worker"); err != nil {
		return reconcile.Result{}, err
	}

	logger.Info("Starting the deletion of worker", "worker", kutil.ObjectName(worker))
	if err := r.actuator.Delete(ctx, worker, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, worker, err, gardencorev1beta1.LastOperationTypeDelete, "Error deleting worker")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, worker, gardencorev1beta1.LastOperationTypeDelete, "Successfully deleted worker"); err != nil {
		return reconcile.Result{}, err
	}

	err := r.removeFinalizerFromWorker(ctx, logger, worker)
	return reconcile.Result{}, err
}

func (r *reconciler) reconcile(ctx context.Context, logger logr.Logger, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, worker, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.statusUpdater.Processing(ctx, worker, operationType, "Reconciling the worker"); err != nil {
		return reconcile.Result{}, err
	}
	logger.Info("Starting the reconciliation of worker", "worker", kutil.ObjectName(worker))
	if err := r.actuator.Reconcile(ctx, worker, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, worker, err, operationType, "Error reconciling worker")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, worker, operationType, "Successfully reconciled worker"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(ctx context.Context, logger logr.Logger, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := r.statusUpdater.Processing(ctx, worker, gardencorev1beta1.LastOperationTypeRestore, "Restoring the worker"); err != nil {
		return reconcile.Result{}, err
	}

	logger.Info("Starting the restoration of worker", "worker", kutil.ObjectName(worker))
	if err := r.actuator.Restore(ctx, worker, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, worker, err, gardencorev1beta1.LastOperationTypeRestore, "Error restoring worker")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, worker, gardencorev1beta1.LastOperationTypeRestore, "Successfully reconciled worker"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.removeAnnotation(ctx, logger, worker); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from worker: %+v", err)
	}

	// requeue to trigger reconciliation
	return reconcile.Result{}, nil
}

func isWorkerMigrated(worker *extensionsv1alpha1.Worker) bool {
	return worker.Status.LastOperation != nil &&
		worker.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate &&
		worker.Status.LastOperation.State == gardencorev1beta1.LastOperationStateSucceeded
}
