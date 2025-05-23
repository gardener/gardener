// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
)

// RequeueAfter is the duration to requeue a controlplane reconciliation if indicated by the actuator.
const RequeueAfter = 2 * time.Second

type reconciler struct {
	actuator      Actuator
	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// controlplane resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return reconcilerutils.OperationAnnotationWrapper(
		mgr,
		func() client.Object { return &extensionsv1alpha1.ControlPlane{} },
		&reconciler{
			actuator:      actuator,
			client:        mgr.GetClient(),
			reader:        mgr.GetAPIReader(),
			statusUpdater: extensionscontroller.NewStatusUpdater(mgr.GetClient()),
		},
	)
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	cp := &extensionsv1alpha1.ControlPlane{}
	if err := r.client.Get(ctx, request.NamespacedName, cp); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, cp.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	if extensionscontroller.IsFailed(cluster) {
		log.Info("Skipping the reconciliation of ControlPlane of failed shoot")
		return reconcile.Result{}, nil
	}

	operationType := v1beta1helper.ComputeOperationType(cp.ObjectMeta, cp.Status.LastOperation)

	switch {
	case extensionscontroller.ShouldSkipOperation(operationType, cp):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, log, cp, cluster)
	case cp.DeletionTimestamp != nil:
		return r.delete(ctx, log, cp, cluster)
	case operationType == gardencorev1beta1.LastOperationTypeRestore:
		return r.restore(ctx, log, cp, cluster)
	default:
		return r.reconcile(ctx, log, cp, cluster, operationType)
	}
}

func (r *reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	operationType gardencorev1beta1.LastOperationType,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(cp, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, cp, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.Processing(ctx, log, cp, operationType, "Reconciling the ControlPlane"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the reconciliation of ControlPlane")
	requeue, err := r.actuator.Reconcile(ctx, log, cp, cluster)
	if err != nil {
		_ = r.statusUpdater.Error(ctx, log, cp, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling ControlPlane")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, cp, operationType, "Successfully reconciled ControlPlane"); err != nil {
		return reconcile.Result{}, err
	}

	if requeue {
		return reconcile.Result{RequeueAfter: RequeueAfter}, nil
	}
	return reconcile.Result{}, nil
}

func (r *reconciler) restore(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(cp, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, cp, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.Processing(ctx, log, cp, gardencorev1beta1.LastOperationTypeRestore, "Restoring the ControlPlane"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the restoration of ControlPlane")
	requeue, err := r.actuator.Restore(ctx, log, cp, cluster)
	if err != nil {
		_ = r.statusUpdater.Error(ctx, log, cp, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Error restoring ControlPlane")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, cp, gardencorev1beta1.LastOperationTypeRestore, "Successfully restored ControlPlane"); err != nil {
		return reconcile.Result{}, err
	}

	if requeue {
		return reconcile.Result{RequeueAfter: RequeueAfter}, nil
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, cp, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from ControlPlane: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (
	reconcile.Result,
	error,
) {
	if err := r.statusUpdater.Processing(ctx, log, cp, gardencorev1beta1.LastOperationTypeMigrate, "Migrating the ControlPlane"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the migration of ControlPlane")
	if err := r.actuator.Migrate(ctx, log, cp, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, log, cp, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeMigrate, "Error migrating ControlPlane")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, cp, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrated ControlPlane"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Removing all finalizers")
	if err := controllerutils.RemoveAllFinalizers(ctx, r.client, cp); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizers: %w", err)
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, cp, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from ControlPlane: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(cp, FinalizerName) {
		log.Info("Deleting ControlPlane causes a no-op as there is no finalizer")
		return reconcile.Result{}, nil
	}

	operationType := v1beta1helper.ComputeOperationType(cp.ObjectMeta, cp.Status.LastOperation)
	if err := r.statusUpdater.Processing(ctx, log, cp, operationType, "Deleting the ControlPlane"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the deletion of ControlPlane")
	var err error
	if cluster != nil && v1beta1helper.ShootNeedsForceDeletion(cluster.Shoot) {
		err = r.actuator.ForceDelete(ctx, log, cp, cluster)
	} else {
		err = r.actuator.Delete(ctx, log, cp, cluster)
	}
	if err != nil {
		_ = r.statusUpdater.Error(ctx, log, cp, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error deleting ControlPlane")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, cp, operationType, "Successfully deleted ControlPlane"); err != nil {
		return reconcile.Result{}, err
	}

	if controllerutil.ContainsFinalizer(cp, FinalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.client, cp, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}
