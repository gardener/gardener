// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerruntime

import (
	"context"
	"fmt"

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

// reconciler reconciles ContainerRuntime resources of Gardener's
// `extensions.gardener.cloud` API group.
type reconciler struct {
	actuator      Actuator
	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// ContainerRuntime resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return reconcilerutils.OperationAnnotationWrapper(
		mgr,
		func() client.Object { return &extensionsv1alpha1.ContainerRuntime{} },
		&reconciler{
			actuator:      actuator,
			client:        mgr.GetClient(),
			reader:        mgr.GetAPIReader(),
			statusUpdater: extensionscontroller.NewStatusUpdater(mgr.GetClient()),
		},
	)
}

// Reconcile is the reconciler function that gets executed in case there are new events for `ContainerRuntime` resources.
func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	cr := &extensionsv1alpha1.ContainerRuntime{}
	if err := r.client.Get(ctx, request.NamespacedName, cr); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, cr.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	if extensionscontroller.IsFailed(cluster) {
		log.Info("Skipping the reconciliation of ContainerRuntime of failed shoot")
		return reconcile.Result{}, nil
	}

	operationType := v1beta1helper.ComputeOperationType(cr.ObjectMeta, cr.Status.LastOperation)

	switch {
	case extensionscontroller.ShouldSkipOperation(operationType, cr):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, log, cr, cluster)
	case cr.DeletionTimestamp != nil:
		return r.delete(ctx, log, cr, cluster)
	case operationType == gardencorev1beta1.LastOperationTypeRestore:
		return r.restore(ctx, log, cr, cluster)
	default:
		return r.reconcile(ctx, log, cr, cluster, operationType)
	}
}

func (r *reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	cr *extensionsv1alpha1.ContainerRuntime,
	cluster *extensionscontroller.Cluster,
	operationType gardencorev1beta1.LastOperationType,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(cr, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, cr, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.Processing(ctx, log, cr, operationType, "Reconciling the ContainerRuntime"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Reconcile(ctx, log, cr, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, log, cr, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling ContainerRuntime")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, cr, operationType, "Successfully reconciled ContainerRuntime"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(
	ctx context.Context,
	log logr.Logger,
	cr *extensionsv1alpha1.ContainerRuntime,
	cluster *extensionscontroller.Cluster,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(cr, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, cr, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.Processing(ctx, log, cr, gardencorev1beta1.LastOperationTypeRestore, "Restoring the ContainerRuntime"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Restore(ctx, log, cr, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, log, cr, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Error restoring ContainerRuntime")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, cr, gardencorev1beta1.LastOperationTypeRestore, "Successfully restored ContainerRuntime"); err != nil {
		return reconcile.Result{}, err
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, cr, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from ContainerRuntime: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	cr *extensionsv1alpha1.ContainerRuntime,
	cluster *extensionscontroller.Cluster,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(cr, FinalizerName) {
		log.Info("Deleting container runtime causes a no-op as there is no finalizer")
		return reconcile.Result{}, nil
	}

	if err := r.statusUpdater.Processing(ctx, log, cr, gardencorev1beta1.LastOperationTypeDelete, "Deleting the ContainerRuntime"); err != nil {
		return reconcile.Result{}, err
	}

	var err error
	if cluster != nil && v1beta1helper.ShootNeedsForceDeletion(cluster.Shoot) {
		err = r.actuator.ForceDelete(ctx, log, cr, cluster)
	} else {
		err = r.actuator.Delete(ctx, log, cr, cluster)
	}
	if err != nil {
		_ = r.statusUpdater.Error(ctx, log, cr, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeDelete, "Error deleting ContainerRuntime")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, cr, gardencorev1beta1.LastOperationTypeDelete, "Successfully deleted ContainerRuntime"); err != nil {
		return reconcile.Result{}, err
	}

	if controllerutil.ContainsFinalizer(cr, FinalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.client, cr, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(
	ctx context.Context,
	log logr.Logger,
	cr *extensionsv1alpha1.ContainerRuntime,
	cluster *extensionscontroller.Cluster,
) (
	reconcile.Result,
	error,
) {
	if err := r.statusUpdater.Processing(ctx, log, cr, gardencorev1beta1.LastOperationTypeMigrate, "Migrating the ContainerRuntime"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Migrate(ctx, log, cr, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, log, cr, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeMigrate, "Error migrating ContainerRuntime")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, cr, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrated ContainerRuntime"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Removing all finalizers")
	if err := controllerutils.RemoveAllFinalizers(ctx, r.client, cr); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizers: %w", err)
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, cr, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from ContainerRuntime: %+v", err)
	}

	return reconcile.Result{}, nil
}
