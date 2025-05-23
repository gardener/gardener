// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

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
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type reconciler struct {
	actuator Actuator

	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// BackupBucket resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return reconcilerutils.OperationAnnotationWrapper(
		mgr,
		func() client.Object { return &extensionsv1alpha1.BackupBucket{} },
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

	bb := &extensionsv1alpha1.BackupBucket{}
	if err := r.client.Get(ctx, request.NamespacedName, bb); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if bb.DeletionTimestamp != nil {
		return r.delete(ctx, log, bb)
	}

	return r.reconcile(ctx, log, bb)
}

func (r *reconciler) reconcile(ctx context.Context, log logr.Logger, bb *extensionsv1alpha1.BackupBucket) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(bb, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, bb, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	operationType := v1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation)
	if err := r.statusUpdater.Processing(ctx, log, bb, operationType, "Reconciling the backupbucket"); err != nil {
		return reconcile.Result{}, err
	}

	secretMetadata, err := kubernetesutils.GetSecretMetadataByReference(ctx, r.client, &bb.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup bucket secret: %+v", err)
	}

	if !controllerutil.ContainsFinalizer(secretMetadata, FinalizerName) {
		log.Info("Adding finalizer to secret", "secret", client.ObjectKeyFromObject(secretMetadata))
		if err := controllerutils.AddFinalizers(ctx, r.client, secretMetadata, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer to secret: %w", err)
		}
	}

	log.Info("Starting the reconciliation of BackupBucket")
	if err := r.actuator.Reconcile(ctx, log, bb); err != nil {
		_ = r.statusUpdater.Error(ctx, log, bb, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling backupbucket")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, bb, operationType, "Successfully reconciled backupbucket"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, log logr.Logger, bb *extensionsv1alpha1.BackupBucket) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(bb, FinalizerName) {
		log.Info("Deleting BackupBucket causes a no-op as there is no finalizer")
		return reconcile.Result{}, nil
	}

	operationType := v1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation)
	if err := r.statusUpdater.Processing(ctx, log, bb, operationType, "Deleting the BackupBucket"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the deletion of BackupBucket")
	if err := r.actuator.Delete(ctx, log, bb); err != nil {
		_ = r.statusUpdater.Error(ctx, log, bb, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error deleting BackupBucket")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, bb, operationType, "Successfully deleted BackupBucket"); err != nil {
		return reconcile.Result{}, err
	}

	secretMetadata, err := kubernetesutils.GetSecretMetadataByReference(ctx, r.client, &bb.Spec.SecretRef)
	if client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup bucket secret: %+v", err)
	}

	if secretMetadata != nil && controllerutil.ContainsFinalizer(secretMetadata, FinalizerName) {
		log.Info("Removing finalizer from secret", "secret", client.ObjectKeyFromObject(secretMetadata))
		if err := controllerutils.RemoveFinalizers(ctx, r.client, secretMetadata, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer from secret: %w", err)
		}
	}

	if controllerutil.ContainsFinalizer(bb, FinalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.client, bb, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}
