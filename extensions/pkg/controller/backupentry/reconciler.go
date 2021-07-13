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

package backupentry

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

type reconciler struct {
	logger   logr.Logger
	actuator Actuator

	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// backupentry resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(actuator Actuator) reconcile.Reconciler {
	logger := log.Log.WithName(ControllerName)

	return extensionscontroller.OperationAnnotationWrapper(
		func() client.Object { return &extensionsv1alpha1.BackupEntry{} },
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
	be := &extensionsv1alpha1.BackupEntry{}
	if err := r.client.Get(ctx, request.NamespacedName, be); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	shootTechnicalID, _ := ExtractShootDetailsFromBackupEntryName(be.Name)
	shoot, err := extensionscontroller.GetShoot(ctx, r.client, shootTechnicalID)
	// As BackupEntry continues to exist post deletion of a Shoot,
	// we do not want to block its deletion when the Cluster is not found.
	if client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, err
	}

	if extensionscontroller.IsShootFailed(shoot) {
		r.logger.Info("Skipping the reconciliation of backupentry of failed shoot", "name", kutil.ObjectName(be))
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(be.ObjectMeta, be.Status.LastOperation)

	switch {
	case extensionscontroller.IsMigrated(be):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, be)
	case be.DeletionTimestamp != nil:
		return r.delete(ctx, be)
	case be.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore:
		return r.restore(ctx, be)
	default:
		return r.reconcile(ctx, be, operationType)
	}
}

func (r *reconciler) reconcile(ctx context.Context, be *extensionsv1alpha1.BackupEntry, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, be, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.statusUpdater.Processing(ctx, be, operationType, "Reconciling the backupentry"); err != nil {
		return reconcile.Result{}, err
	}

	secret, err := kutil.GetSecretByReference(ctx, r.client, &be.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup entry secret: %+v", err)
	}
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, secret, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to ensure finalizer on backup entry secret: %+v", err)
	}

	r.logger.Info("Starting the reconciliation of backupentry", "backupentry", kutil.ObjectName(be))
	if err := r.actuator.Reconcile(ctx, be); err != nil {
		_ = r.statusUpdater.Error(ctx, be, extensionscontroller.ReconcileErrCauseOrErr(err), operationType, "Error reconciling backupentry")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, be, operationType, "Successfully reconciled backupentry"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(ctx context.Context, be *extensionsv1alpha1.BackupEntry) (reconcile.Result, error) {
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, be, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.statusUpdater.Processing(ctx, be, gardencorev1beta1.LastOperationTypeRestore, "Restoring the backupentry"); err != nil {
		return reconcile.Result{}, err
	}

	secret, err := kutil.GetSecretByReference(ctx, r.client, &be.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup entry secret: %+v", err)
	}
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, secret, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to ensure finalizer on backup entry secret: %+v", err)
	}

	r.logger.Info("Starting the restoration of backupentry", "backupentry", kutil.ObjectName(be))
	if err := r.actuator.Restore(ctx, be); err != nil {
		_ = r.statusUpdater.Error(ctx, be, extensionscontroller.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Error restoring backupentry")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, be, gardencorev1beta1.LastOperationTypeRestore, "Successfully restored backupentry"); err != nil {
		return reconcile.Result{}, err
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, be, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove the annotation from backupentry: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, be *extensionsv1alpha1.BackupEntry) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(be, FinalizerName) {
		r.logger.Info("Deleting backupentry causes a no-op as there is no finalizer", "backupentry", kutil.ObjectName(be))
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(be.ObjectMeta, be.Status.LastOperation)
	if err := r.statusUpdater.Processing(ctx, be, operationType, "Deleting the backupentry"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the deletion of backupentry", "backupentry", kutil.ObjectName(be))

	secret, err := kutil.GetSecretByReference(ctx, r.client, &be.Spec.SecretRef)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("failed to get backup entry secret: %+v", err)
		}

		r.logger.Info("Skipping deletion as referred secret does not exist any more - removing finalizer", "backupentry", kutil.ObjectName(be))
		if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, be, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("error removing finalizer from backupentry: %+v", err)
		}

		return reconcile.Result{}, nil
	}

	if err := r.actuator.Delete(ctx, be); err != nil {
		_ = r.statusUpdater.Error(ctx, be, extensionscontroller.ReconcileErrCauseOrErr(err), operationType, "Error deleting backupentry")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, be, operationType, "Successfully deleted backupentry"); err != nil {
		return reconcile.Result{}, err
	}

	if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, secret, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer on backup entry secret: %+v", err)
	}

	r.logger.Info("Removing finalizer.", "backupentry", be.Name)
	if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, be, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizer from backupentry: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(ctx context.Context, be *extensionsv1alpha1.BackupEntry) (reconcile.Result, error) {
	if err := r.statusUpdater.Processing(ctx, be, gardencorev1beta1.LastOperationTypeMigrate, "Migrating the backupentry"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the migration of backupentry", "backupentry", kutil.ObjectName(be))
	if err := r.actuator.Migrate(ctx, be); err != nil {
		_ = r.statusUpdater.Error(ctx, be, extensionscontroller.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeMigrate, "Error migrating backupentry")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, be, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrated backupentry"); err != nil {
		return reconcile.Result{}, err
	}

	secret, err := kutil.GetSecretByReference(ctx, r.client, &be.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup entry secret: %+v", err)
	}
	if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, secret, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer on backup entry secret: %+v", err)
	}

	r.logger.Info("Removing all finalizers", "backupentry", kutil.ObjectName(be))
	if err := extensionscontroller.DeleteAllFinalizers(ctx, r.client, be); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing all finalizers from backupentry: %+v", err)
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, be, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from backupentry: %+v", err)
	}

	return reconcile.Result{}, nil
}
