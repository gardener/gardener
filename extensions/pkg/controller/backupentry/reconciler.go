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

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

type reconciler struct {
	actuator Actuator

	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// backupentry resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(actuator Actuator) reconcile.Reconciler {
	return reconcilerutils.OperationAnnotationWrapper(
		func() client.Object { return &extensionsv1alpha1.BackupEntry{} },
		&reconciler{
			actuator:      actuator,
			statusUpdater: extensionscontroller.NewStatusUpdater(),
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
	log := logf.FromContext(ctx)

	be := &extensionsv1alpha1.BackupEntry{}
	if err := r.client.Get(ctx, request.NamespacedName, be); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	shootTechnicalID, _ := ExtractShootDetailsFromBackupEntryName(be.Name)
	cluster, err := extensionscontroller.GetCluster(ctx, r.client, shootTechnicalID)
	// As BackupEntry continues to exist post deletion of a Shoot,
	// we do not want to block its deletion when the Cluster is not found.
	if client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, err
	}

	if extensionscontroller.IsFailed(cluster) {
		log.Info("Skipping the reconciliation of BackupEntry of failed shoot")
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(be.ObjectMeta, be.Status.LastOperation)

	if cluster != nil && cluster.Shoot != nil && operationType != gardencorev1beta1.LastOperationTypeMigrate {
		key := "backupentry:" + be.Name
		ok, watchdogCtx, cleanup, err := common.GetOwnerCheckResultAndContext(ctx, r.client, shootTechnicalID, cluster.Shoot.Name, key)
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
	case extensionscontroller.ShouldSkipOperation(operationType, be):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, log, be)
	case be.DeletionTimestamp != nil:
		return r.delete(ctx, log, be)
	case operationType == gardencorev1beta1.LastOperationTypeRestore:
		return r.restore(ctx, log, be)
	default:
		return r.reconcile(ctx, log, be, operationType)
	}
}

func (r *reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	be *extensionsv1alpha1.BackupEntry,
	operationType gardencorev1beta1.LastOperationType,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(be, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, be, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.Processing(ctx, log, be, operationType, "Reconciling the BackupEntry"); err != nil {
		return reconcile.Result{}, err
	}

	secret, err := kutil.GetSecretByReference(ctx, r.client, &be.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup entry secret: %+v", err)
	}

	if !controllerutil.ContainsFinalizer(secret, FinalizerName) {
		log.Info("Adding finalizer to secret", "secret", client.ObjectKeyFromObject(secret))
		if err := controllerutils.AddFinalizers(ctx, r.client, secret, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer to secret: %w", err)
		}
	}

	log.Info("Starting the reconciliation of BackupEntry")
	if err := r.actuator.Reconcile(ctx, log, be); err != nil {
		_ = r.statusUpdater.Error(ctx, log, be, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling BackupEntry")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, be, operationType, "Successfully reconciled BackupEntry"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(ctx context.Context, log logr.Logger, be *extensionsv1alpha1.BackupEntry) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(be, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, be, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if err := r.statusUpdater.Processing(ctx, log, be, gardencorev1beta1.LastOperationTypeRestore, "Restoring the BackupEntry"); err != nil {
		return reconcile.Result{}, err
	}

	secret, err := kutil.GetSecretByReference(ctx, r.client, &be.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup entry secret: %+v", err)
	}

	if !controllerutil.ContainsFinalizer(secret, FinalizerName) {
		log.Info("Adding finalizer to secret", "secret", client.ObjectKeyFromObject(secret))
		if err := controllerutils.AddFinalizers(ctx, r.client, secret, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer to secret: %w", err)
		}
	}

	log.Info("Starting the restoration of BackupEntry")
	if err := r.actuator.Restore(ctx, log, be); err != nil {
		_ = r.statusUpdater.Error(ctx, log, be, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Error restoring BackupEntry")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, be, gardencorev1beta1.LastOperationTypeRestore, "Successfully restored BackupEntry"); err != nil {
		return reconcile.Result{}, err
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, be, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove the annotation from BackupEntry: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, log logr.Logger, be *extensionsv1alpha1.BackupEntry) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(be, FinalizerName) {
		log.Info("Deleting BackupEntry causes a no-op as there is no finalizer")
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(be.ObjectMeta, be.Status.LastOperation)
	if err := r.statusUpdater.Processing(ctx, log, be, operationType, "Deleting the BackupEntry"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the deletion of BackupEntry")

	secret, err := kutil.GetSecretByReference(ctx, r.client, &be.Spec.SecretRef)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("failed to get backup entry secret: %+v", err)
		}

		log.Info("Skipping deletion as referred secret does not exist any more")

		if controllerutil.ContainsFinalizer(be, FinalizerName) {
			log.Info("Removing finalizer")
			if err := controllerutils.RemoveFinalizers(ctx, r.client, be, FinalizerName); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}

		return reconcile.Result{}, nil
	}

	if err := r.actuator.Delete(ctx, log, be); err != nil {
		_ = r.statusUpdater.Error(ctx, log, be, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error deleting BackupEntry")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, be, operationType, "Successfully deleted BackupEntry"); err != nil {
		return reconcile.Result{}, err
	}

	if controllerutil.ContainsFinalizer(secret, FinalizerName) {
		log.Info("Removing finalizer from secret", "secret", client.ObjectKeyFromObject(secret))
		if err := controllerutils.RemoveFinalizers(ctx, r.client, secret, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer from secret: %w", err)
		}
	}

	if controllerutil.ContainsFinalizer(be, FinalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.client, be, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(ctx context.Context, log logr.Logger, be *extensionsv1alpha1.BackupEntry) (reconcile.Result, error) {
	if err := r.statusUpdater.Processing(ctx, log, be, gardencorev1beta1.LastOperationTypeMigrate, "Migrating the BackupEntry"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the migration of BackupEntry")
	if err := r.actuator.Migrate(ctx, log, be); err != nil {
		_ = r.statusUpdater.Error(ctx, log, be, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeMigrate, "Error migrating BackupEntry")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, be, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrated BackupEntry"); err != nil {
		return reconcile.Result{}, err
	}

	secret, err := kutil.GetSecretByReference(ctx, r.client, &be.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup entry secret: %+v", err)
	}

	if controllerutil.ContainsFinalizer(secret, FinalizerName) {
		log.Info("Removing finalizer from secret", "secret", client.ObjectKeyFromObject(secret))
		if err := controllerutils.RemoveFinalizers(ctx, r.client, secret, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer from secret: %w", err)
		}
	}

	log.Info("Removing all finalizers")
	if err := controllerutils.RemoveAllFinalizers(ctx, r.client, be); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing all finalizers: %w", err)
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, be, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from BackupEntry: %+v", err)
	}

	return reconcile.Result{}, nil
}
