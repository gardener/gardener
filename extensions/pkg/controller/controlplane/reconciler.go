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

package controlplane

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// RequeueAfter is the duration to requeue a controlplane reconciliation if indicated by the actuator.
const RequeueAfter = 2 * time.Second

type reconciler struct {
	logger   logr.Logger
	actuator Actuator

	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// controlplane resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(actuator Actuator) reconcile.Reconciler {
	logger := log.Log.WithName(ControllerName)

	return extensionscontroller.OperationAnnotationWrapper(
		func() client.Object { return &extensionsv1alpha1.ControlPlane{} },
		&reconciler{
			logger:        logger,
			actuator:      actuator,
			statusUpdater: extensionscontroller.NewStatusUpdater(logger),
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
	cp := &extensionsv1alpha1.ControlPlane{}
	if err := r.client.Get(ctx, request.NamespacedName, cp); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, cp.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	if extensionscontroller.IsFailed(cluster) {
		r.logger.Info("Skipping the reconciliation of controlplane of failed shoot.", "controlplane", kutil.ObjectName(cp))
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(cp.ObjectMeta, cp.Status.LastOperation)

	switch {
	case extensionscontroller.IsMigrated(cp):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, cp, cluster)
	case cp.DeletionTimestamp != nil:
		return r.delete(ctx, cp, cluster)
	case cp.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore:
		return r.restore(ctx, cp, cluster)
	default:
		return r.reconcile(ctx, cp, cluster, operationType)
	}
}

func (r *reconciler) reconcile(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, cp, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.statusUpdater.Processing(ctx, cp, operationType, "Reconciling the controlplane"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the reconciliation of controlplane", "controlplane", kutil.ObjectName(cp))
	requeue, err := r.actuator.Reconcile(ctx, cp, cluster)
	if err != nil {
		_ = r.statusUpdater.Error(ctx, cp, extensionscontroller.ReconcileErrCauseOrErr(err), operationType, "Error reconciling controlplane")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, cp, operationType, "Successfully reconciled controlplane"); err != nil {
		return reconcile.Result{}, err
	}

	if requeue {
		return reconcile.Result{RequeueAfter: RequeueAfter}, nil
	}
	return reconcile.Result{}, nil
}

func (r *reconciler) restore(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, cp, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.statusUpdater.Processing(ctx, cp, gardencorev1beta1.LastOperationTypeRestore, "Restoring the controlplane"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the restoration of controlplane", "controlplane", kutil.ObjectName(cp))
	requeue, err := r.actuator.Restore(ctx, cp, cluster)
	if err != nil {
		_ = r.statusUpdater.Error(ctx, cp, extensionscontroller.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Error restoring controlplane")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, cp, gardencorev1beta1.LastOperationTypeRestore, "Successfully restored controlplane"); err != nil {
		return reconcile.Result{}, err
	}

	if requeue {
		return reconcile.Result{RequeueAfter: RequeueAfter}, nil
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, cp, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from controlplane: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := r.statusUpdater.Processing(ctx, cp, gardencorev1beta1.LastOperationTypeMigrate, "Migrating the controlplane"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the migration of controlplane", "controlplane", kutil.ObjectName(cp))
	if err := r.actuator.Migrate(ctx, cp, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, cp, extensionscontroller.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeMigrate, "Error migrating controlplane")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, cp, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrated controlplane"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing all finalizers.", "controlplane", kutil.ObjectName(cp))
	if err := extensionscontroller.DeleteAllFinalizers(ctx, r.client, cp); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizers from controlplane: %+v", err)
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, cp, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from controlplane: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(cp, FinalizerName) {
		r.logger.Info("Deleting controlplane causes a no-op as there is no finalizer", "controlplane", kutil.ObjectName(cp))
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(cp.ObjectMeta, cp.Status.LastOperation)
	if err := r.statusUpdater.Processing(ctx, cp, operationType, "Deleting the controlplane"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the deletion of controlplane", "controlplane", kutil.ObjectName(cp))
	if err := r.actuator.Delete(ctx, cp, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, cp, extensionscontroller.ReconcileErrCauseOrErr(err), operationType, "Error deleting controlplane")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, cp, operationType, "Successfully deleted controlplane"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing finalizer", "controlplane", kutil.ObjectName(cp))
	if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, cp, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizer from controlplane: %+v", err)
	}

	return reconcile.Result{}, nil
}
