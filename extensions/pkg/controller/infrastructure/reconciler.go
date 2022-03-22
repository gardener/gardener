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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
	configValidator ConfigValidator

	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// infrastructure resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(actuator Actuator, configValidator ConfigValidator) reconcile.Reconciler {
	logger := log.Log.WithName(ControllerName)

	return reconcilerutils.OperationAnnotationWrapper(
		func() client.Object { return &extensionsv1alpha1.Infrastructure{} },
		&reconciler{
			logger:          logger,
			actuator:        actuator,
			configValidator: configValidator,
			statusUpdater:   extensionscontroller.NewStatusUpdater(logger),
		},
	)
}

func (r *reconciler) InjectFunc(f inject.Func) error {
	if r.configValidator != nil {
		if err := f(r.configValidator); err != nil {
			return err
		}
	}
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

	logger := r.logger.WithValues("infrastructure", kutil.ObjectName(infrastructure))
	if extensionscontroller.IsFailed(cluster) {
		logger.Info("Skipping the reconciliation of infrastructure of failed shoot")
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(infrastructure.ObjectMeta, infrastructure.Status.LastOperation)

	if cluster.Shoot != nil && operationType != gardencorev1beta1.LastOperationTypeMigrate {
		key := "infrastructure:" + kutil.ObjectName(infrastructure)
		ok, watchdogCtx, cleanup, err := common.GetOwnerCheckResultAndContext(ctx, r.client, infrastructure.Namespace, cluster.Shoot.Name, key)
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
	case extensionscontroller.ShouldSkipOperation(operationType, infrastructure):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, logger.WithValues("operation", "migrate"), infrastructure, cluster)
	case infrastructure.DeletionTimestamp != nil:
		return r.delete(ctx, logger.WithValues("operation", "delete"), infrastructure, cluster)
	case operationType == gardencorev1beta1.LastOperationTypeRestore:
		return r.restore(ctx, logger.WithValues("operation", "restore"), infrastructure, cluster)
	default:
		return r.reconcile(ctx, logger.WithValues("operation", "reconcile"), infrastructure, cluster, operationType)
	}
}

func (r *reconciler) reconcile(ctx context.Context, logger logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, infrastructure, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.statusUpdater.Processing(ctx, infrastructure, operationType, "Reconciling the infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.validateConfig(ctx, infrastructure); err != nil {
		_ = r.statusUpdater.Error(ctx, infrastructure, err, operationType, "Error checking infrastructure config")
		return reconcile.Result{}, err
	}

	logger.Info("Starting the reconciliation of infrastructure", "infrastructure", kutil.ObjectName(infrastructure))
	if err := r.actuator.Reconcile(ctx, infrastructure, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, infrastructure, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling infrastructure")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, infrastructure, operationType, "Successfully reconciled infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, logger logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(infrastructure, FinalizerName) {
		logger.Info("Deleting infrastructure causes a no-op as there is no finalizer", "infrastructure", kutil.ObjectName(infrastructure))
		return reconcile.Result{}, nil
	}

	if err := r.statusUpdater.Processing(ctx, infrastructure, gardencorev1beta1.LastOperationTypeDelete, "Deleting the infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Delete(ctx, infrastructure, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, infrastructure, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeDelete, "Error deleting infrastructure")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, infrastructure, gardencorev1beta1.LastOperationTypeDelete, "Successfully deleted infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	err := r.removeFinalizerFromInfrastructure(ctx, logger, infrastructure)
	return reconcile.Result{}, err
}

func (r *reconciler) migrate(ctx context.Context, logger logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := r.statusUpdater.Processing(ctx, infrastructure, gardencorev1beta1.LastOperationTypeMigrate, "Starting Migration of the infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Migrate(ctx, infrastructure, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, infrastructure, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeMigrate, "Error migrating infrastructure")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, infrastructure, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrated infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.removeFinalizerFromInfrastructure(ctx, logger, infrastructure); err != nil {
		return reconcile.Result{}, err
	}

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

	if err := r.statusUpdater.Processing(ctx, infrastructure, gardencorev1beta1.LastOperationTypeRestore, "Restoring the infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.validateConfig(ctx, infrastructure); err != nil {
		_ = r.statusUpdater.Error(ctx, infrastructure, err, gardencorev1beta1.LastOperationTypeRestore, "Error checking infrastructure config")
		return reconcile.Result{}, err
	}

	if err := r.actuator.Restore(ctx, infrastructure, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, infrastructure, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Error restoring infrastructure")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.removeAnnotation(ctx, logger, infrastructure); err != nil {
		return reconcile.Result{}, err
	}

	err := r.statusUpdater.Success(ctx, infrastructure, gardencorev1beta1.LastOperationTypeRestore, "Successfully restored infrastructure")
	return reconcile.Result{}, err
}

func (r *reconciler) removeFinalizerFromInfrastructure(ctx context.Context, logger logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure) error {
	logger.Info("Removing finalizer", "infrastructure", kutil.ObjectName(infrastructure))
	if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, infrastructure, FinalizerName); err != nil {
		return fmt.Errorf("error removing finalizer from infrastructure: %+v", err)
	}
	return nil
}

func (r *reconciler) removeAnnotation(ctx context.Context, logger logr.Logger, infrastructure *extensionsv1alpha1.Infrastructure) error {
	logger.Info("Removing operation annotation")
	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, infrastructure, v1beta1constants.GardenerOperation); err != nil {
		return fmt.Errorf("error removing annotation from infrastructure: %+v", err)
	}
	return nil
}

func (r *reconciler) validateConfig(ctx context.Context, infrastructure *extensionsv1alpha1.Infrastructure) error {
	if r.configValidator == nil {
		return nil
	}

	if allErrs := r.configValidator.Validate(ctx, infrastructure); len(allErrs) > 0 {
		if filteredErrs := allErrs.Filter(field.NewErrorTypeMatcher(field.ErrorTypeInternal)); len(filteredErrs) < len(allErrs) {
			return allErrs.ToAggregate()
		}
		return gardencorev1beta1helper.NewErrorWithCodes(allErrs.ToAggregate(), gardencorev1beta1.ErrorConfigurationProblem)
	}

	return nil
}
