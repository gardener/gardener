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
	"errors"
	"fmt"
	"strconv"
	"time"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const reconcilerName = "backupentry"

// reconciler implements the reconcile.Reconcile interface for backupEntry reconciliation.
type reconciler struct {
	clientMap clientmap.ClientMap
	recorder  record.EventRecorder
	config    *config.GardenletConfiguration
}

// newReconciler returns the new backupEntry reconciler.
func newReconciler(clientMap clientmap.ClientMap, config *config.GardenletConfiguration, recorder record.EventRecorder) reconcile.Reconciler {
	return &reconciler{
		clientMap: clientMap,
		recorder:  recorder,
		config:    config,
	}
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	gardenClient, err := r.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	be := &gardencorev1beta1.BackupEntry{}
	if err := gardenClient.Client().Get(ctx, request.NamespacedName, be); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// Remove the operation annotation if its value is not "restore"
	// If it's "restore", it will be removed at the end of the reconciliation since it's needed
	// to properly determine that the operation is "restore, and not "reconcile"
	if operationType, ok := be.Annotations[v1beta1constants.GardenerOperation]; ok && operationType != v1beta1constants.GardenerOperationRestore {
		if updateErr := removeGardenerOperationAnnotation(ctx, gardenClient.Client(), be); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not remove %q annotation: %w", v1beta1constants.GardenerOperation, updateErr)
		}
	}

	if be.DeletionTimestamp != nil {
		return r.deleteBackupEntry(ctx, log, gardenClient, be)
	}

	if shouldMigrateBackupEntry(be) {
		return r.migrateBackupEntry(ctx, log, gardenClient, be)
	}

	if !IsBackupEntryManagedByThisGardenlet(be, r.config) {
		log.V(1).Info("Skipping because BackupEntry is not managed by this gardenlet", "seedName", *be.Spec.SeedName)
		return reconcile.Result{}, nil
	}

	// When a BackupEntry deletion timestamp is not set we need to create/reconcile the backup entry.
	return r.reconcileBackupEntry(ctx, log, gardenClient, be)
}

func (r *reconciler) reconcileBackupEntry(
	ctx context.Context,
	log logr.Logger,
	gardenClient kubernetes.Interface,
	backupEntry *gardencorev1beta1.BackupEntry,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(backupEntry, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, gardenClient.Client(), backupEntry, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(backupEntry.ObjectMeta, backupEntry.Status.LastOperation)
	if updateErr := r.updateBackupEntryStatusOperationStart(ctx, gardenClient.Client(), backupEntry, operationType); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after reconciliation start: %w", updateErr)
	}

	seedClient, err := r.clientMap.GetClient(ctx, keys.ForSeedWithName(*backupEntry.Spec.SeedName))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get seed client: %w", err)
	}

	a := newActuator(log, gardenClient.Client(), seedClient.Client(), backupEntry)
	if err := a.Reconcile(ctx); err != nil {
		log.Error(err, "Failed to reconcile")

		reconcileErr := &gardencorev1beta1.LastError{
			Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}
		r.recorder.Eventf(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", reconcileErr.Description)
		if updateErr := updateBackupEntryStatusError(ctx, gardenClient.Client(), backupEntry, operationType, reconcileErr); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not update status after reconciliation error: %w", updateErr)
		}
		return reconcile.Result{}, errors.New(reconcileErr.Description)
	}

	if updateErr := updateBackupEntryStatusSucceeded(ctx, gardenClient.Client(), backupEntry, operationType); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after reconciliation success: %w", updateErr)
	}

	if kutil.HasMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRestore) {
		if updateErr := removeGardenerOperationAnnotation(ctx, gardenClient.Client(), backupEntry); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not remove %q annotation: %w", v1beta1constants.GardenerOperation, updateErr)
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) deleteBackupEntry(
	ctx context.Context,
	log logr.Logger,
	gardenClient kubernetes.Interface,
	backupEntry *gardencorev1beta1.BackupEntry,
) (
	reconcile.Result,
	error,
) {
	if !sets.NewString(backupEntry.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		log.V(1).Info("Do not need to do anything as the BackupEntry does not have my finalizer")
		return reconcile.Result{}, nil
	}

	gracePeriod := computeGracePeriod(*r.config.Controllers.BackupEntry.DeletionGracePeriodHours, r.config.Controllers.BackupEntry.DeletionGracePeriodShootPurposes, gardencore.ShootPurpose(backupEntry.Annotations[v1beta1constants.ShootPurpose]))
	present, _ := strconv.ParseBool(backupEntry.ObjectMeta.Annotations[gardencorev1beta1.BackupEntryForceDeletion])
	if present || time.Since(backupEntry.DeletionTimestamp.Local()) > gracePeriod {
		operationType := gardencorev1beta1helper.ComputeOperationType(backupEntry.ObjectMeta, backupEntry.Status.LastOperation)
		if updateErr := r.updateBackupEntryStatusOperationStart(ctx, gardenClient.Client(), backupEntry, operationType); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not update status after deletion start: %w", updateErr)
		}

		seedClient, err := r.clientMap.GetClient(ctx, keys.ForSeedWithName(*backupEntry.Spec.SeedName))
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to get seed client: %w", err)
		}

		a := newActuator(log, gardenClient.Client(), seedClient.Client(), backupEntry)
		if err := a.Delete(ctx); err != nil {
			log.Error(err, "Failed to delete")

			deleteErr := &gardencorev1beta1.LastError{
				Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
				Description: err.Error(),
			}
			r.recorder.Eventf(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, "%s", deleteErr.Description)

			if updateErr := updateBackupEntryStatusError(ctx, gardenClient.Client(), backupEntry, operationType, deleteErr); updateErr != nil {
				return reconcile.Result{}, fmt.Errorf("could not update status after deletion error: %w", updateErr)
			}
			return reconcile.Result{}, errors.New(deleteErr.Description)
		}

		if updateErr := updateBackupEntryStatusSucceeded(ctx, gardenClient.Client(), backupEntry, operationType); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not update status after deletion success: %w", updateErr)
		}

		log.Info("Successfully deleted, removing finalizer")
		return reconcile.Result{}, controllerutils.PatchRemoveFinalizers(ctx, gardenClient.Client(), backupEntry, gardencorev1beta1.GardenerName)
	}

	if updateErr := updateBackupEntryStatusPending(ctx, gardenClient.Client(), backupEntry, fmt.Sprintf("Deletion of backup entry is scheduled for %s", backupEntry.DeletionTimestamp.Time.Add(gracePeriod))); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after deletion success: %w", updateErr)
	}

	return reconcile.Result{}, nil
}

func shouldMigrateBackupEntry(be *gardencorev1beta1.BackupEntry) bool {
	return be.Status.SeedName != nil && be.Spec.SeedName != nil && *be.Spec.SeedName != *be.Status.SeedName
}

func (r *reconciler) migrateBackupEntry(
	ctx context.Context,
	log logr.Logger,
	gardenClient kubernetes.Interface,
	backupEntry *gardencorev1beta1.BackupEntry,
) (
	reconcile.Result,
	error,
) {
	if !sets.NewString(backupEntry.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		log.V(1).Info("Do not need to do anything as the BackupEntry does not have my finalizer")
		return reconcile.Result{}, nil
	}

	if updateErr := r.updateBackupEntryStatusOperationStart(ctx, gardenClient.Client(), backupEntry, gardencorev1beta1.LastOperationTypeMigrate); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after migration start: %w", updateErr)
	}

	seedClient, err := r.clientMap.GetClient(ctx, keys.ForSeedWithName(*backupEntry.Status.SeedName))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get seed client: %w", err)
	}

	a := newActuator(log, gardenClient.Client(), seedClient.Client(), backupEntry)
	if err := a.Migrate(ctx); err != nil {
		log.Error(err, "Failed to migrate")

		reconcileErr := &gardencorev1beta1.LastError{
			Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}
		r.recorder.Eventf(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", reconcileErr.Description)

		if updateErr := updateBackupEntryStatusError(ctx, gardenClient.Client(), backupEntry, gardencorev1beta1.LastOperationTypeMigrate, reconcileErr); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not update status after migration error: %w", updateErr)
		}
		return reconcile.Result{}, errors.New(reconcileErr.Description)
	}

	if updateErr := updateBackupEntryStatusSucceeded(ctx, gardenClient.Client(), backupEntry, gardencorev1beta1.LastOperationTypeMigrate); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after migration success: %w", updateErr)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) updateBackupEntryStatusOperationStart(ctx context.Context, c client.StatusClient, be *gardencorev1beta1.BackupEntry, operationType gardencorev1beta1.LastOperationType) error {
	var description string
	switch operationType {
	case gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationTypeReconcile:
		description = "Reconciliation of BackupEntry state initialized."

	case gardencorev1beta1.LastOperationTypeRestore:
		description = "Restoration of BackupEntry state initialized."

	case gardencorev1beta1.LastOperationTypeMigrate:
		description = "Migration of BackupEntry state initialized."

	case gardencorev1beta1.LastOperationTypeDelete:
		description = "Deletion of BackupEntry state initialized."
	}

	patch := client.MergeFrom(be.DeepCopy())

	be.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           operationType,
		State:          gardencorev1beta1.LastOperationStateProcessing,
		Progress:       0,
		Description:    description,
		LastUpdateTime: metav1.Now(),
	}
	be.Status.ObservedGeneration = be.Generation
	if be.Status.SeedName == nil {
		be.Status.SeedName = be.Spec.SeedName
	}

	return c.Status().Patch(ctx, be, patch)
}

func updateBackupEntryStatusError(ctx context.Context, c client.StatusClient, be *gardencorev1beta1.BackupEntry, operationType gardencorev1beta1.LastOperationType, lastError *gardencorev1beta1.LastError) error {
	patch := client.MergeFrom(be.DeepCopy())

	var progress int32 = 1
	if be.Status.LastOperation != nil {
		progress = be.Status.LastOperation.Progress
	}
	be.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           operationType,
		State:          gardencorev1beta1.LastOperationStateError,
		Progress:       progress,
		Description:    lastError.Description + " Operation will be retried.",
		LastUpdateTime: metav1.Now(),
	}
	be.Status.LastError = lastError

	return c.Status().Patch(ctx, be, patch)
}

func updateBackupEntryStatusSucceeded(ctx context.Context, c client.StatusClient, be *gardencorev1beta1.BackupEntry, operationType gardencorev1beta1.LastOperationType) error {
	var description string

	switch operationType {
	case gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationTypeReconcile:
		description = "Reconciliation of BackupEntry succeeded."

	case gardencorev1beta1.LastOperationTypeRestore:
		description = "Restoration of BackupEntry succeeded."

	case gardencorev1beta1.LastOperationTypeMigrate:
		description = "Migration of BackupEntry succeeded."

	case gardencorev1beta1.LastOperationTypeDelete:
		description = "Deletion of BackupEntry succeeded."
	}

	patch := client.MergeFrom(be.DeepCopy())

	be.Status.LastError = nil
	be.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           operationType,
		State:          gardencorev1beta1.LastOperationStateSucceeded,
		Progress:       100,
		Description:    description,
		LastUpdateTime: metav1.Now(),
	}
	if operationType == gardencorev1beta1.LastOperationTypeMigrate {
		be.Status.SeedName = nil
	}

	return c.Status().Patch(ctx, be, patch)
}

func updateBackupEntryStatusPending(ctx context.Context, c client.StatusClient, be *gardencorev1beta1.BackupEntry, message string) error {
	patch := client.MergeFrom(be.DeepCopy())

	be.Status.ObservedGeneration = be.Generation
	be.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           gardencorev1beta1helper.ComputeOperationType(be.ObjectMeta, be.Status.LastOperation),
		State:          gardencorev1beta1.LastOperationStatePending,
		Progress:       0,
		Description:    message,
		LastUpdateTime: metav1.Now(),
	}

	return c.Status().Patch(ctx, be, patch)
}

func computeGracePeriod(deletionGracePeriodHours int, deletionGracePeriodShootPurposes []gardencore.ShootPurpose, shootPurpose gardencore.ShootPurpose) time.Duration {
	// If no dedicated list of purposes is provided then the grace period applies for all purposes. If the shoot purpose
	// is empty then it was not yet updated with the purpose annotation or the corresponding `Shoot` is already deleted
	// from the system. In this case, for backwards-compatibility, the grace period applies as well.
	if len(deletionGracePeriodShootPurposes) == 0 || len(shootPurpose) == 0 {
		return time.Hour * time.Duration(deletionGracePeriodHours)
	}

	// Otherwise, the grace period only applies for the purposes in the list.
	for _, p := range deletionGracePeriodShootPurposes {
		if p == shootPurpose {
			return time.Hour * time.Duration(deletionGracePeriodHours)
		}
	}

	// If the shoot purpose was not found in the list then the grace period does not apply.
	return 0
}

func removeGardenerOperationAnnotation(ctx context.Context, c client.Client, be *gardencorev1beta1.BackupEntry) error {
	patch := client.MergeFrom(be.DeepCopy())
	delete(be.GetAnnotations(), v1beta1constants.GardenerOperation)
	return c.Patch(ctx, be, patch)
}
