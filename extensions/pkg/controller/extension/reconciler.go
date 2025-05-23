// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

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
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// reconciler reconciles Extension resources of Gardener's
// `extensions.gardener.cloud` API group.
type reconciler struct {
	actuator Actuator

	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater

	resync        time.Duration
	finalizerName string
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// Extension resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, args AddArgs) reconcile.Reconciler {
	return reconcilerutils.OperationAnnotationWrapper(
		mgr,
		func() client.Object { return &extensionsv1alpha1.Extension{} },
		&reconciler{
			actuator:      args.Actuator,
			client:        mgr.GetClient(),
			reader:        mgr.GetAPIReader(),
			statusUpdater: extensionscontroller.NewStatusUpdater(mgr.GetClient()),
			finalizerName: fmt.Sprintf("%s/%s", FinalizerPrefix, args.FinalizerSuffix),
			resync:        args.Resync,
		},
	)
}

// Reconcile is the reconciler function that gets executed in case there are new events for `Extension` resources.
func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ex := &extensionsv1alpha1.Extension{}
	if err := r.client.Get(ctx, request.NamespacedName, ex); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	var cluster *extensions.Cluster
	if gardenerutils.IsShootNamespace(ex.Namespace) {
		var err error
		cluster, err = extensionscontroller.GetCluster(ctx, r.client, ex.Namespace)
		if err != nil {
			return reconcile.Result{}, err
		}

		if extensionscontroller.IsFailed(cluster) {
			log.Info("Skipping the reconciliation of Extension of failed shoot")
			return reconcile.Result{}, nil
		}
	}

	operationType := v1beta1helper.ComputeOperationType(ex.ObjectMeta, ex.Status.LastOperation)

	switch {
	case extensionscontroller.ShouldSkipOperation(operationType, ex):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, log, ex)
	case ex.DeletionTimestamp != nil:
		return r.delete(ctx, log, ex, cluster != nil && v1beta1helper.ShootNeedsForceDeletion(cluster.Shoot))
	case operationType == gardencorev1beta1.LastOperationTypeRestore:
		return r.restore(ctx, log, ex, operationType)
	default:
		if result, err := r.reconcile(ctx, log, ex, operationType); err != nil {
			return result, err
		}
		return reconcile.Result{Requeue: r.resync != 0, RequeueAfter: r.resync}, nil
	}
}

func (r *reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	ex *extensionsv1alpha1.Extension,
	operationType gardencorev1beta1.LastOperationType,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(ex, r.finalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, ex, r.finalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.Processing(ctx, log, ex, operationType, "Reconciling the Extension"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the reconciliation of Extension")
	if err := r.actuator.Reconcile(ctx, log, ex); err != nil {
		_ = r.statusUpdater.Error(ctx, log, ex, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling Extension")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, ex, operationType, "Successfully reconciled Extension"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension, forceDelete bool) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(ex, r.finalizerName) {
		log.Info("Deleting Extension causes a no-op as there is no finalizer")
		return reconcile.Result{}, nil
	}

	if err := r.statusUpdater.Processing(ctx, log, ex, gardencorev1beta1.LastOperationTypeDelete, "Deleting the Extension"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the deletion of Extension")
	var err error
	if forceDelete {
		err = r.actuator.ForceDelete(ctx, log, ex)
	} else {
		err = r.actuator.Delete(ctx, log, ex)
	}
	if err != nil {
		_ = r.statusUpdater.Error(ctx, log, ex, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeDelete, "Error deleting the Extension")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, ex, gardencorev1beta1.LastOperationTypeDelete, "Successfully deleted the Extension"); err != nil {
		return reconcile.Result{}, err
	}

	if controllerutil.ContainsFinalizer(ex, r.finalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.client, ex, r.finalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(
	ctx context.Context,
	log logr.Logger,
	ex *extensionsv1alpha1.Extension,
	operationType gardencorev1beta1.LastOperationType,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(ex, r.finalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, ex, r.finalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.Processing(ctx, log, ex, operationType, "Restoring Extension resource"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the restoration of extension")
	if err := r.actuator.Restore(ctx, log, ex); err != nil {
		_ = r.statusUpdater.Error(ctx, log, ex, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Unable to restore Extension resource")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, ex, operationType, "Successfully restored Extension resource"); err != nil {
		return reconcile.Result{}, err
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, ex, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from Extension resource: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) (reconcile.Result, error) {
	if err := r.statusUpdater.Processing(ctx, log, ex, gardencorev1beta1.LastOperationTypeMigrate, "Migrate Extension resource."); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the migration of extension")
	if err := r.actuator.Migrate(ctx, log, ex); err != nil {
		_ = r.statusUpdater.Error(ctx, log, ex, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeMigrate, "Error migrating Extension resource")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, ex, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrated Extension resource"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Removing all finalizers")
	if err := controllerutils.RemoveAllFinalizers(ctx, r.client, ex); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizers: %w", err)
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, ex, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from Extension resource: %+v", err)
	}

	return reconcile.Result{}, nil
}
