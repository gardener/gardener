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

package infrastructure

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

const (
	// EventInfrastructureReconciliation an event reason to describe infrastructure reconciliation.
	EventInfrastructureReconciliation string = "InfrastructureReconciliation"
	// EventInfrastructureDeletion an event reason to describe infrastructure deletion.
	EventInfrastructureDeletion string = "InfrastructureDeletion"
	// EventInfrastructureMigration an event reason to describe infrastructure migration.
	EventInfrastructureMigration string = "InfrastructureMigration"
	// EventInfrastructureRestoration an event reason to describe infrastructure restoration.
	EventInfrastructureRestoration string = "InfrastructureRestoration"
)

type reconciler struct {
	logger   logr.Logger
	actuator Actuator

	client   client.Client
	reader   client.Reader
	recorder record.EventRecorder
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// infrastructure resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return extensionscontroller.OperationAnnotationWrapper(
		func() client.Object { return &extensionsv1alpha1.Infrastructure{} },
		&reconciler{
			logger:   log.Log.WithName(ControllerName),
			actuator: actuator,
			recorder: mgr.GetEventRecorderFor(ControllerName),
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

func (r *reconciler) InjectAPIReader(reader client.Reader) error {
	r.reader = reader
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	infrastructure := &extensionsv1alpha1.Infrastructure{}
	if err := r.client.Get(ctx, request.NamespacedName, infrastructure); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, infrastructure.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	logger := r.logger.WithValues("infrastructure", client.ObjectKeyFromObject(infrastructure))
	if extensionscontroller.IsFailed(cluster) {
		r.logger.Info("Stop reconciling Infrastructure of failed Shoot.", "namespace", request.Namespace, "name", infrastructure.Name)
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(infrastructure.ObjectMeta, infrastructure.Status.LastOperation)

	switch {
	case extensionscontroller.IsMigrated(infrastructure):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, logger.WithValues("operation", "migrate"), infrastructure, cluster)
	case infrastructure.DeletionTimestamp != nil:
		return r.delete(ctx, logger.WithValues("operation", "delete"), infrastructure, cluster)
	case infrastructure.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore:
		return r.restore(ctx, logger.WithValues("operation", "restore"), infrastructure, cluster)
	default:
		return r.reconcile(ctx, logger.WithValues("operation", "reconcile"), infrastructure, cluster, operationType)
	}
}

func (r *reconciler) reconcile(ctx context.Context, logger logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	logger.Info("Ensuring finalizer")
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, infrastructure, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusProcessing(ctx, logger, infrastructure, operationType, EventInfrastructureReconciliation, "Reconciling the infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Reconcile(ctx, infrastructure, cluster); err != nil {
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), infrastructure, operationType, EventInfrastructureReconciliation, "Error reconciling infrastructure"))
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, logger, infrastructure, operationType, EventInfrastructureReconciliation, "Successfully reconciled infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, logger logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(infrastructure, FinalizerName) {
		logger.Info("Deleting infrastructure causes a no-op as there is no finalizer.")
		return reconcile.Result{}, nil
	}

	if err := r.updateStatusProcessing(ctx, logger, infrastructure, gardencorev1beta1.LastOperationTypeDelete, EventInfrastructureDeletion, "Deleting the infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Delete(ctx, infrastructure, cluster); err != nil {
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), infrastructure, gardencorev1beta1.LastOperationTypeDelete, EventInfrastructureDeletion, "Error deleting infrastructure"))
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, logger, infrastructure, gardencorev1beta1.LastOperationTypeDelete, EventInfrastructureDeletion, "Successfully deleted infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	err := r.removeFinalizerFromInfrastructure(ctx, logger, infrastructure)
	return reconcile.Result{}, err
}

func (r *reconciler) migrate(ctx context.Context, logger logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := r.updateStatusProcessing(ctx, logger, infrastructure, gardencorev1beta1.LastOperationTypeMigrate, EventInfrastructureMigration, "Starting Migration of the Infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Migrate(ctx, infrastructure, cluster); err != nil {
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), infrastructure, gardencorev1beta1.LastOperationTypeMigrate, EventInfrastructureMigration, "Error migrating infrastructure"))
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, logger, infrastructure, gardencorev1beta1.LastOperationTypeMigrate, EventInfrastructureMigration, "Successfully migrated Infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.removeFinalizerFromInfrastructure(ctx, logger, infrastructure); err != nil {
		return reconcile.Result{}, err
	}

	// remove operation annotation 'migrate'
	if err := r.removeAnnotation(ctx, logger, infrastructure); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(ctx context.Context, logger logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	logger.Info("Ensuring finalizer")
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, infrastructure, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusProcessing(ctx, logger, infrastructure, gardencorev1beta1.LastOperationTypeRestore, EventInfrastructureRestoration, "Restoring the infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Restore(ctx, infrastructure, cluster); err != nil {
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), infrastructure, gardencorev1beta1.LastOperationTypeRestore, EventInfrastructureRestoration, "Error restoring infrastructure"))
		return extensionscontroller.ReconcileErr(err)
	}

	// remove operation annotation 'restore'
	if err := r.removeAnnotation(ctx, logger, infrastructure); err != nil {
		return reconcile.Result{}, err
	}

	err := r.updateStatusSuccess(ctx, logger, infrastructure, gardencorev1beta1.LastOperationTypeRestore, EventInfrastructureRestoration, "Successfully restored infrastructure")
	return reconcile.Result{}, err
}

func (r *reconciler) updateStatusProcessing(ctx context.Context, logger logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, lastOperationType gardencorev1beta1.LastOperationType, event, description string) error {
	logger.Info(description)
	r.recorder.Eventf(infrastructure, corev1.EventTypeNormal, event, description)
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, infrastructure, func() error {
		infrastructure.Status.LastOperation = extensionscontroller.LastOperation(lastOperationType, gardencorev1beta1.LastOperationStateProcessing, 1, description)
		return nil
	})
}

func (r *reconciler) updateStatusError(ctx context.Context, err error, infrastructure *extensionsv1alpha1.Infrastructure, lastOperationType gardencorev1beta1.LastOperationType, event, description string) error {
	r.recorder.Eventf(infrastructure, corev1.EventTypeWarning, event, fmt.Sprintf("%s: %+v", description, err))
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, infrastructure, func() error {
		infrastructure.Status.ObservedGeneration = infrastructure.Generation
		infrastructure.Status.LastOperation, infrastructure.Status.LastError = extensionscontroller.ReconcileError(lastOperationType, gardencorev1beta1helper.FormatLastErrDescription(fmt.Errorf("%s: %v", description, err)), 50, gardencorev1beta1helper.ExtractErrorCodes(gardencorev1beta1helper.DetermineError(err, err.Error()))...)
		return nil
	})
}

func (r *reconciler) updateStatusSuccess(ctx context.Context, logger logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, lastOperationType gardencorev1beta1.LastOperationType, event, description string) error {
	logger.Info(description)
	r.recorder.Eventf(infrastructure, corev1.EventTypeNormal, event, description)
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, infrastructure, func() error {
		infrastructure.Status.ObservedGeneration = infrastructure.Generation
		infrastructure.Status.LastOperation, infrastructure.Status.LastError = extensionscontroller.ReconcileSucceeded(lastOperationType, description)
		return nil
	})
}

func (r *reconciler) removeFinalizerFromInfrastructure(ctx context.Context, logger logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure) error {
	logger.Info("Removing finalizer")
	if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, infrastructure, FinalizerName); err != nil {
		msg := fmt.Sprintf("error removing finalizer from Infrastructure: %+v", err)
		r.recorder.Eventf(infrastructure, corev1.EventTypeWarning, EventInfrastructureMigration, msg)
		return fmt.Errorf(msg)
	}
	return nil
}

func (r *reconciler) removeAnnotation(ctx context.Context, logger logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure) error {
	logger.Info("Removing operation annotation")
	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, infrastructure, v1beta1constants.GardenerOperation); err != nil {
		msg := fmt.Sprintf("error removing annotation from Infrastructure: %+v", err)
		r.recorder.Eventf(infrastructure, corev1.EventTypeWarning, EventInfrastructureMigration, msg)
		return fmt.Errorf(msg)
	}
	return nil
}
