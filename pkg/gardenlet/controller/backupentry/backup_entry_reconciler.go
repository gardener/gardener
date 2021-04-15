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
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// reconciler implements the reconcile.Reconcile interface for backupEntry reconciliation.
type reconciler struct {
	clientMap clientmap.ClientMap
	recorder  record.EventRecorder
	logger    *logrus.Logger
	config    *config.GardenletConfiguration
}

// newReconciler returns the new backupBucker reconciler.
func newReconciler(clientMap clientmap.ClientMap, recorder record.EventRecorder, config *config.GardenletConfiguration) reconcile.Reconciler {
	return &reconciler{
		clientMap: clientMap,
		recorder:  recorder,
		logger:    logger.Logger,
		config:    config,
	}
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	gardenClient, err := r.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	be := &gardencorev1beta1.BackupEntry{}
	if err := gardenClient.Client().Get(ctx, request.NamespacedName, be); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Debugf("[BACKUPENTRY RECONCILE] %s - skipping because BackupEntry has been deleted", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("[BACKUPENTRY RECONCILE] %s - unable to retrieve object from store: %v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	if be.DeletionTimestamp != nil {
		return r.deleteBackupEntry(ctx, gardenClient, be)
	}

	if shouldMigrateBackupEntry(be) {
		return r.migrateBackupEntry(ctx, gardenClient, be)
	}

	if !controllerutils.BackupEntryIsManagedByThisGardenlet(ctx, gardenClient.Client(), be, r.config) {
		r.logger.Debugf("Skipping because BackupEntry is not managed by this gardenlet in seed %s", *be.Spec.SeedName)
		return reconcile.Result{}, nil
	}

	// When a BackupEntry deletion timestamp is not set we need to create/reconcile the backup entry.
	return r.reconcileBackupEntry(ctx, gardenClient, be)
}

func (r *reconciler) reconcileBackupEntry(ctx context.Context, gardenClient kubernetes.Interface, backupEntry *gardencorev1beta1.BackupEntry) (reconcile.Result, error) {
	backupEntryLogger := logger.NewFieldLogger(logger.Logger, "backupentry", backupEntry.Name)

	if !controllerutil.ContainsFinalizer(backupEntry, gardencorev1beta1.GardenerName) {
		if err := controllerutils.PatchAddFinalizers(ctx, gardenClient.Client(), backupEntry, gardencorev1beta1.GardenerName); err != nil {
			backupEntryLogger.Errorf("Failed to ensure gardener finalizer on backupentry: %+v", err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(backupEntry.ObjectMeta, backupEntry.Status.LastOperation)
	if kutil.HasMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRestore) {
		operationType = gardencorev1beta1.LastOperationTypeRestore
	}

	if updateErr := r.updateBackupEntryStatusOperationStart(ctx, gardenClient.DirectClient(), backupEntry, operationType); updateErr != nil {
		backupEntryLogger.Errorf("Could not update the status after reconciliation start: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	seedClient, err := r.clientMap.GetClient(ctx, keys.ForSeedWithName(*backupEntry.Spec.SeedName))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get seed client: %w", err)
	}

	a := newActuator(gardenClient, seedClient, backupEntry, r.logger)
	if err := a.Reconcile(ctx); err != nil {
		backupEntryLogger.Errorf("Failed to reconcile backup entry: %+v", err)

		reconcileErr := &gardencorev1beta1.LastError{
			Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}
		r.recorder.Eventf(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", reconcileErr.Description)
		if updateErr := updateBackupEntryStatusError(ctx, gardenClient.DirectClient(), backupEntry, operationType, reconcileErr); updateErr != nil {
			backupEntryLogger.Errorf("Could not update the BackupEntry status after reconciliation error: %+v", updateErr)
			return reconcile.Result{}, updateErr
		}
		return reconcile.Result{}, errors.New(reconcileErr.Description)
	}

	if updateErr := updateBackupEntryStatusSucceeded(ctx, gardenClient.DirectClient(), backupEntry, operationType); updateErr != nil {
		backupEntryLogger.Errorf("Could not update the BackupEntry status after reconciliation success: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	if updateErr := controllerutils.RemoveGardenerOperationAnnotation(ctx, retry.DefaultBackoff, gardenClient.DirectClient(), backupEntry); updateErr != nil {
		backupEntryLogger.Errorf("Could not remove %q annotation: %+v", v1beta1constants.GardenerOperation, updateErr)
		return reconcile.Result{}, updateErr
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) deleteBackupEntry(ctx context.Context, gardenClient kubernetes.Interface, backupEntry *gardencorev1beta1.BackupEntry) (reconcile.Result, error) {
	backupEntryLogger := logger.NewFieldLogger(r.logger, "backupentry", backupEntry.Name)
	if !sets.NewString(backupEntry.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		backupEntryLogger.Debug("Do not need to do anything as the BackupEntry does not have my finalizer")
		return reconcile.Result{}, nil
	}

	gracePeriod := computeGracePeriod(*r.config.Controllers.BackupEntry.DeletionGracePeriodHours, r.config.Controllers.BackupEntry.DeletionGracePeriodShootPurposes, gardencore.ShootPurpose(backupEntry.Annotations[v1beta1constants.ShootPurpose]))
	present, _ := strconv.ParseBool(backupEntry.ObjectMeta.Annotations[gardencorev1beta1.BackupEntryForceDeletion])
	if present || time.Since(backupEntry.DeletionTimestamp.Local()) > gracePeriod {
		operationType := gardencorev1beta1helper.ComputeOperationType(backupEntry.ObjectMeta, backupEntry.Status.LastOperation)
		if updateErr := r.updateBackupEntryStatusOperationStart(ctx, gardenClient.DirectClient(), backupEntry, operationType); updateErr != nil {
			backupEntryLogger.Errorf("Could not update the BackupEntry status after deletion start: %+v", updateErr)
			return reconcile.Result{}, updateErr
		}

		seedClient, err := r.clientMap.GetClient(ctx, keys.ForSeedWithName(*backupEntry.Spec.SeedName))
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to get seed client: %w", err)
		}

		a := newActuator(gardenClient, seedClient, backupEntry, r.logger)
		if err := a.Delete(ctx); err != nil {
			backupEntryLogger.Errorf("Failed to delete backup entry: %+v", err)

			deleteErr := &gardencorev1beta1.LastError{
				Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
				Description: err.Error(),
			}
			r.recorder.Eventf(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, "%s", deleteErr.Description)

			if updateErr := updateBackupEntryStatusError(ctx, gardenClient.DirectClient(), backupEntry, operationType, deleteErr); updateErr != nil {
				backupEntryLogger.Errorf("Could not update the BackupEntry status after deletion error: %+v", updateErr)
				return reconcile.Result{}, updateErr
			}
			return reconcile.Result{}, errors.New(deleteErr.Description)
		}
		if updateErr := updateBackupEntryStatusSucceeded(ctx, gardenClient.DirectClient(), backupEntry, operationType); updateErr != nil {
			backupEntryLogger.Errorf("Could not update the BackupEntry status after deletion successful: %+v", updateErr)
			return reconcile.Result{}, updateErr
		}
		backupEntryLogger.Infof("Successfully deleted backup entry %q", backupEntry.Name)
		return reconcile.Result{}, controllerutils.PatchRemoveFinalizers(ctx, gardenClient.Client(), backupEntry, gardencorev1beta1.GardenerName)
	}
	if updateErr := updateBackupEntryStatusPending(ctx, gardenClient.DirectClient(), backupEntry, fmt.Sprintf("Deletion of backup entry is scheduled for %s", backupEntry.DeletionTimestamp.Time.Add(gracePeriod))); updateErr != nil {
		backupEntryLogger.Errorf("Could not update the BackupEntry status after deletion successful: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}
	return reconcile.Result{}, nil
}

func shouldMigrateBackupEntry(be *gardencorev1beta1.BackupEntry) bool {
	return be.Status.SeedName != nil && be.Spec.SeedName != nil && *be.Spec.SeedName != *be.Status.SeedName
}

func (r *reconciler) migrateBackupEntry(ctx context.Context, gardenClient kubernetes.Interface, backupEntry *gardencorev1beta1.BackupEntry) (reconcile.Result, error) {
	backupEntryLogger := logger.NewFieldLogger(r.logger, "backupentry", backupEntry.Name)
	if !sets.NewString(backupEntry.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		backupEntryLogger.Debug("Do not need to do anything as the BackupEntry does not have my finalizer")
		return reconcile.Result{}, nil
	}

	if updateErr := r.updateBackupEntryStatusOperationStart(ctx, gardenClient.DirectClient(), backupEntry, gardencorev1beta1.LastOperationTypeMigrate); updateErr != nil {
		backupEntryLogger.Errorf("Could not update the status after migration start: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	seedClient, err := r.clientMap.GetClient(ctx, keys.ForSeedWithName(*backupEntry.Status.SeedName))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get seed client: %w", err)
	}

	a := newActuator(gardenClient, seedClient, backupEntry, r.logger)
	if err := a.Migrate(ctx); err != nil {
		backupEntryLogger.Errorf("Failed to migrate backup entry: %+v", err)

		reconcileErr := &gardencorev1beta1.LastError{
			Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}
		r.recorder.Eventf(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", reconcileErr.Description)

		if updateErr := updateBackupEntryStatusError(ctx, gardenClient.DirectClient(), backupEntry, gardencorev1beta1.LastOperationTypeMigrate, reconcileErr); updateErr != nil {
			backupEntryLogger.Errorf("Could not update the BackupEntry status after migration error: %+v", updateErr)
			return reconcile.Result{}, updateErr
		}
		return reconcile.Result{}, errors.New(reconcileErr.Description)
	}

	if updateErr := updateBackupEntryStatusSucceeded(ctx, gardenClient.DirectClient(), backupEntry, gardencorev1beta1.LastOperationTypeMigrate); updateErr != nil {
		backupEntryLogger.Errorf("Could not update the BackupEntry status after migration success: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	if updateErr := controllerutils.RemoveGardenerOperationAnnotation(ctx, retry.DefaultBackoff, gardenClient.DirectClient(), backupEntry); updateErr != nil {
		backupEntryLogger.Errorf("Could not remove %q annotation: %+v", v1beta1constants.GardenerOperation, updateErr)
		return reconcile.Result{}, updateErr
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) updateBackupEntryStatusOperationStart(ctx context.Context, c client.Client, be *gardencorev1beta1.BackupEntry, operationType gardencorev1beta1.LastOperationType) error {
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

	return kutil.TryUpdateStatus(ctx, retry.DefaultBackoff, c, be, func() error {
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
		return nil
	})
}

func updateBackupEntryStatusError(ctx context.Context, c client.Client, be *gardencorev1beta1.BackupEntry, operationType gardencorev1beta1.LastOperationType, lastError *gardencorev1beta1.LastError) error {
	return kutil.TryUpdateStatus(ctx, retry.DefaultBackoff, c, be, func() error {
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
		return nil
	})
}

func updateBackupEntryStatusSucceeded(ctx context.Context, c client.Client, be *gardencorev1beta1.BackupEntry, operationType gardencorev1beta1.LastOperationType) error {
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

	return kutil.TryUpdateStatus(ctx, retry.DefaultBackoff, c, be, func() error {
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
		return nil
	})
}

func updateBackupEntryStatusPending(ctx context.Context, c client.Client, be *gardencorev1beta1.BackupEntry, message string) error {
	return kutil.TryUpdateStatus(ctx, retry.DefaultBackoff, c, be, func() error {
		be.Status.ObservedGeneration = be.Generation
		be.Status.LastOperation = &gardencorev1beta1.LastOperation{
			Type:           gardencorev1beta1helper.ComputeOperationType(be.ObjectMeta, be.Status.LastOperation),
			State:          gardencorev1beta1.LastOperationStatePending,
			Progress:       0,
			Description:    message,
			LastUpdateTime: metav1.Now(),
		}
		return nil
	})
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
