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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	contextutil "github.com/gardener/gardener/pkg/utils/context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

const (
	// EventBackupEntryReconciliation an event reason to describe backup entry reconciliation.
	EventBackupEntryReconciliation string = "BackupEntryReconciliation"
	// EventBackupEntryDeletion an event reason to describe backup entry deletion.
	EventBackupEntryDeletion string = "BackupEntryDeletion"
	// EventBackupEntryMigration an event reason to describe backup entry migration.
	EventBackupEntryMigration string = "BackupEntryMigration"
	// EventBackupEntryRestoration an event reason to describe backup entry restoration.
	EventBackupEntryRestoration string = "BackupEntryRestoration"
)

type reconciler struct {
	logger   logr.Logger
	actuator Actuator

	ctx      context.Context
	client   client.Client
	recorder record.EventRecorder
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// backupentry resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return extensionscontroller.OperationAnnotationWrapper(
		&extensionsv1alpha1.BackupEntry{},
		&reconciler{
			logger:   log.Log.WithName(ControllerName),
			actuator: actuator,
			recorder: mgr.GetEventRecorderFor(ControllerName),
		})
}

func (r *reconciler) InjectFunc(f inject.Func) error {
	return f(r.actuator)
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

func (r *reconciler) InjectStopChannel(stopCh <-chan struct{}) error {
	r.ctx = contextutil.FromStopChannel(stopCh)
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	be := &extensionsv1alpha1.BackupEntry{}
	if err := r.client.Get(r.ctx, request.NamespacedName, be); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	shootTechnicalID, _ := ExtractShootDetailsFromBackupEntryName(be.Name)
	shoot, err := extensionscontroller.GetShoot(r.ctx, r.client, shootTechnicalID)
	// As BackupEntry continues to exist post deletion of a Shoot,
	// we do not want to block its deletion when the Cluster is not found.
	if client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, err
	}

	if extensionscontroller.IsShootFailed(shoot) {
		r.logger.Info("Stop reconciling BackupEntry of failed Shoot.", "name", be.Name)
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(be.ObjectMeta, be.Status.LastOperation)

	switch {
	case extensionscontroller.IsMigrated(be):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(r.ctx, be)
	case be.DeletionTimestamp != nil:
		return r.delete(r.ctx, be)
	case be.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore:
		return r.restore(r.ctx, be)
	default:
		return r.reconcile(r.ctx, be, operationType)
	}
}

func (r *reconciler) reconcile(ctx context.Context, be *extensionsv1alpha1.BackupEntry, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := extensionscontroller.EnsureFinalizer(ctx, r.client, FinalizerName, be); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusProcessing(ctx, be, operationType, "Reconciling the backupentry"); err != nil {
		return reconcile.Result{}, err
	}

	secret, err := extensionscontroller.GetSecretByReference(ctx, r.client, &be.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup entry secret: %+v", err)
	}
	if err := extensionscontroller.EnsureFinalizer(ctx, r.client, FinalizerName, secret); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to ensure finalizer on backup entry secret: %+v", err)
	}

	r.logger.Info("Starting the reconciliation of backupentry", "backupentry", be.Name)
	r.recorder.Event(be, corev1.EventTypeNormal, EventBackupEntryReconciliation, "Reconciling the backupentry")
	if err := r.actuator.Reconcile(ctx, be); err != nil {
		msg := "Error reconciling backupentry"
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), be, operationType, msg)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully reconciled backupentry"
	r.logger.Info(msg, "backupentry", be.Name)
	r.recorder.Event(be, corev1.EventTypeNormal, EventBackupEntryReconciliation, msg)
	if err := r.updateStatusSuccess(ctx, be, operationType, msg); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(ctx context.Context, be *extensionsv1alpha1.BackupEntry) (reconcile.Result, error) {
	if err := extensionscontroller.EnsureFinalizer(ctx, r.client, FinalizerName, be); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusProcessing(ctx, be, gardencorev1beta1.LastOperationTypeRestore, "Restoring the backupentry"); err != nil {
		return reconcile.Result{}, err
	}

	secret, err := extensionscontroller.GetSecretByReference(ctx, r.client, &be.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup entry secret: %+v", err)
	}
	if err := extensionscontroller.EnsureFinalizer(ctx, r.client, FinalizerName, secret); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to ensure finalizer on backup entry secret: %+v", err)
	}

	r.logger.Info("Starting the restoration of backupentry", "backupentry", be.Name)
	r.recorder.Event(be, corev1.EventTypeNormal, EventBackupEntryRestoration, "Restoring the backupentry")
	if err := r.actuator.Restore(ctx, be); err != nil {
		msg := "Error restoring backupentry"
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), be, gardencorev1beta1.LastOperationTypeRestore, msg)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully restored backupentry"
	r.logger.Info(msg, "backupentry", be.Name)
	r.recorder.Event(be, corev1.EventTypeNormal, EventBackupEntryRestoration, msg)
	if err := r.updateStatusSuccess(ctx, be, gardencorev1beta1.LastOperationTypeRestore, msg); err != nil {
		return reconcile.Result{}, err
	}
	// remove operation annotation 'restore'
	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, be, v1beta1constants.GardenerOperation); err != nil {
		msg := "Error removing annotation from BackupEntry"
		r.recorder.Eventf(be, corev1.EventTypeWarning, EventBackupEntryMigration, "%s: %+v", msg, err)
		return reconcile.Result{}, fmt.Errorf("%s: %+v", msg, err)
	}
	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, be *extensionsv1alpha1.BackupEntry) (reconcile.Result, error) {
	hasFinalizer, err := extensionscontroller.HasFinalizer(be, FinalizerName)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not instantiate finalizer deletion: %+v", err)
	}
	if !hasFinalizer {
		r.logger.Info("Deleting backupentry causes a no-op as there is no finalizer.", "backupentry", be.Name)
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(be.ObjectMeta, be.Status.LastOperation)
	if err := r.updateStatusProcessing(ctx, be, operationType, "Deleting the backupentry"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the deletion of backupentry", "backupentry", be.Name)
	r.recorder.Event(be, corev1.EventTypeNormal, EventBackupEntryDeletion, "Deleting the backupentry")
	if err := r.actuator.Delete(r.ctx, be); err != nil {
		msg := "Error deleting backupentry"
		r.recorder.Eventf(be, corev1.EventTypeWarning, EventBackupEntryDeletion, "%s: %+v", msg, err)
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), be, operationType, msg)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully deleted backupentry"
	r.logger.Info(msg, "backupentry", be.Name)
	r.recorder.Event(be, corev1.EventTypeNormal, EventBackupEntryDeletion, msg)
	if err := r.updateStatusSuccess(ctx, be, operationType, msg); err != nil {
		return reconcile.Result{}, err
	}

	secret, err := extensionscontroller.GetSecretByReference(ctx, r.client, &be.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup entry secret: %+v", err)
	}
	if err := extensionscontroller.DeleteFinalizer(ctx, r.client, FinalizerName, secret); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer on backup entry secret: %+v", err)
	}

	r.logger.Info("Removing finalizer.", "backupentry", be.Name)
	if err := extensionscontroller.DeleteFinalizer(ctx, r.client, FinalizerName, be); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizer from backupentry: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(ctx context.Context, be *extensionsv1alpha1.BackupEntry) (reconcile.Result, error) {
	if err := r.updateStatusProcessing(ctx, be, gardencorev1beta1.LastOperationTypeMigrate, "Migrating the backupentry"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the migration of backupentry", "backupentry", be.Name)
	r.recorder.Event(be, corev1.EventTypeNormal, EventBackupEntryMigration, "Migrating the backupentry")
	if err := r.actuator.Migrate(r.ctx, be); err != nil {
		msg := "Error migrating backupentry"
		r.recorder.Eventf(be, corev1.EventTypeWarning, EventBackupEntryMigration, "%s: %+v", msg, err)
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), be, gardencorev1beta1.LastOperationTypeMigrate, msg)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully migrated backupentry"
	r.logger.Info(msg, "backupentry", be.Name)
	r.recorder.Event(be, corev1.EventTypeNormal, EventBackupEntryMigration, msg)
	if err := r.updateStatusSuccess(ctx, be, gardencorev1beta1.LastOperationTypeMigrate, msg); err != nil {
		return reconcile.Result{}, err
	}

	secret, err := extensionscontroller.GetSecretByReference(ctx, r.client, &be.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup entry secret: %+v", err)
	}
	if err := extensionscontroller.DeleteFinalizer(ctx, r.client, FinalizerName, secret); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer on backup entry secret: %+v", err)
	}

	r.logger.Info("Removing all finalizers.", "backupentry", be.Name)
	if err := extensionscontroller.DeleteAllFinalizers(ctx, r.client, be); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing all finalizers from backupentry: %+v", err)
	}

	// remove operation annotation 'migrate'
	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, be, v1beta1constants.GardenerOperation); err != nil {
		msg := "Error removing annotation from BackupEntry"
		r.recorder.Eventf(be, corev1.EventTypeWarning, EventBackupEntryMigration, "%s: %+v", msg, err)
		return reconcile.Result{}, fmt.Errorf("%s: %+v", msg, err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) updateStatusProcessing(ctx context.Context, be *extensionsv1alpha1.BackupEntry, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, be, func() error {
		be.Status.LastOperation = extensionscontroller.LastOperation(lastOperationType, gardencorev1beta1.LastOperationStateProcessing, 1, description)
		return nil
	})
}

func (r *reconciler) updateStatusError(ctx context.Context, err error, be *extensionsv1alpha1.BackupEntry, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, be, func() error {
		be.Status.ObservedGeneration = be.Generation
		be.Status.LastOperation, be.Status.LastError = extensionscontroller.ReconcileError(lastOperationType, gardencorev1beta1helper.FormatLastErrDescription(fmt.Errorf("%s: %v", description, err)), 50, gardencorev1beta1helper.ExtractErrorCodes(err)...)
		return nil
	})
}

func (r *reconciler) updateStatusSuccess(ctx context.Context, be *extensionsv1alpha1.BackupEntry, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, be, func() error {
		be.Status.ObservedGeneration = be.Generation
		be.Status.LastOperation, be.Status.LastError = extensionscontroller.ReconcileSucceeded(lastOperationType, description)
		return nil
	})
}
