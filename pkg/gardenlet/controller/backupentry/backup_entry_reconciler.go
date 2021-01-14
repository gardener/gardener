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

	"sigs.k8s.io/controller-runtime/pkg/client"

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
	// When a BackupEntry deletion timestamp is not set we need to create/reconcile the backup entry.
	return r.reconcileBackupEntry(ctx, gardenClient, be)
}

func (r *reconciler) reconcileBackupEntry(ctx context.Context, gardenClient kubernetes.Interface, backupEntry *gardencorev1beta1.BackupEntry) (reconcile.Result, error) {
	backupEntryLogger := logger.NewFieldLogger(logger.Logger, "backupentry", backupEntry.Name)

	if err := controllerutils.EnsureFinalizer(ctx, gardenClient.DirectClient(), backupEntry, gardencorev1beta1.GardenerName); err != nil {
		backupEntryLogger.Errorf("Failed to ensure gardener finalizer on backupentry: %+v", err)
		return reconcile.Result{}, err
	}

	if updateErr := r.updateBackupEntryStatusProcessing(ctx, gardenClient.DirectClient(), backupEntry, "Reconciliation of Backup Entry state in progress.", 2); updateErr != nil {
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

		if updateErr := updateBackupEntryStatusError(ctx, gardenClient.DirectClient(), backupEntry, reconcileErr.Description+" Operation will be retried.", reconcileErr); updateErr != nil {
			backupEntryLogger.Errorf("Could not update the BackupEntry status after deletion error: %+v", updateErr)
			return reconcile.Result{}, updateErr
		}
		return reconcile.Result{}, errors.New(reconcileErr.Description)
	}

	if updateErr := updateBackupEntryStatusSucceeded(ctx, gardenClient.DirectClient(), backupEntry, "Backup Entry has been successfully reconciled."); updateErr != nil {
		backupEntryLogger.Errorf("Could not update the Shoot status after reconciliation success: %+v", updateErr)
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

	gracePeriod := computeGracePeriod(*r.config.Controllers.BackupEntry.DeletionGracePeriodHours)
	present, _ := strconv.ParseBool(backupEntry.ObjectMeta.Annotations[gardencorev1beta1.BackupEntryForceDeletion])
	if present || time.Since(backupEntry.DeletionTimestamp.Local()) > gracePeriod {
		if updateErr := r.updateBackupEntryStatusProcessing(ctx, gardenClient.DirectClient(), backupEntry, "Deletion of Backup Entry in progress.", 2); updateErr != nil {
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

			if updateErr := updateBackupEntryStatusError(ctx, gardenClient.DirectClient(), backupEntry, deleteErr.Description+" Operation will be retried.", deleteErr); updateErr != nil {
				backupEntryLogger.Errorf("Could not update the BackupEntry status after deletion error: %+v", updateErr)
				return reconcile.Result{}, updateErr
			}
			return reconcile.Result{}, errors.New(deleteErr.Description)
		}
		if updateErr := updateBackupEntryStatusSucceeded(ctx, gardenClient.DirectClient(), backupEntry, "Backup Entry has been successfully deleted."); updateErr != nil {
			backupEntryLogger.Errorf("Could not update the BackupEntry status after deletion successful: %+v", updateErr)
			return reconcile.Result{}, updateErr
		}
		backupEntryLogger.Infof("Successfully deleted backup entry %q", backupEntry.Name)
		return reconcile.Result{}, controllerutils.RemoveGardenerFinalizer(ctx, gardenClient.DirectClient(), backupEntry)
	}
	if updateErr := updateBackupEntryStatusPending(ctx, gardenClient.DirectClient(), backupEntry, fmt.Sprintf("Deletion of backup entry is scheduled for %s", backupEntry.DeletionTimestamp.Time.Add(gracePeriod))); updateErr != nil {
		backupEntryLogger.Errorf("Could not update the BackupEntry status after deletion successful: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}
	return reconcile.Result{}, nil
}

func (r *reconciler) updateBackupEntryStatusProcessing(ctx context.Context, c client.Client, be *gardencorev1beta1.BackupEntry, message string, progress int32) error {
	return kutil.TryUpdateStatus(ctx, retry.DefaultBackoff, c, be, func() error {
		be.Status.LastOperation = &gardencorev1beta1.LastOperation{
			Type:           gardencorev1beta1helper.ComputeOperationType(be.ObjectMeta, be.Status.LastOperation),
			State:          gardencorev1beta1.LastOperationStateProcessing,
			Progress:       progress,
			Description:    message,
			LastUpdateTime: metav1.Now(),
		}
		return nil
	})
}

func updateBackupEntryStatusError(ctx context.Context, c client.Client, be *gardencorev1beta1.BackupEntry, message string, lastError *gardencorev1beta1.LastError) error {
	return kutil.TryUpdateStatus(ctx, retry.DefaultBackoff, c, be, func() error {
		var progress int32 = 1
		if be.Status.LastOperation != nil {
			progress = be.Status.LastOperation.Progress
		}
		be.Status.LastOperation = &gardencorev1beta1.LastOperation{
			Type:           gardencorev1beta1helper.ComputeOperationType(be.ObjectMeta, be.Status.LastOperation),
			State:          gardencorev1beta1.LastOperationStateError,
			Progress:       progress,
			Description:    message,
			LastUpdateTime: metav1.Now(),
		}
		be.Status.LastError = lastError
		return nil
	})
}

func updateBackupEntryStatusSucceeded(ctx context.Context, c client.Client, be *gardencorev1beta1.BackupEntry, message string) error {
	return kutil.TryUpdateStatus(ctx, retry.DefaultBackoff, c, be, func() error {
		be.Status.LastError = nil
		be.Status.LastOperation = &gardencorev1beta1.LastOperation{
			Type:           gardencorev1beta1helper.ComputeOperationType(be.ObjectMeta, be.Status.LastOperation),
			State:          gardencorev1beta1.LastOperationStateSucceeded,
			Progress:       100,
			Description:    message,
			LastUpdateTime: metav1.Now(),
		}
		be.Status.ObservedGeneration = be.Generation
		return nil
	})
}

func updateBackupEntryStatusPending(ctx context.Context, c client.Client, be *gardencorev1beta1.BackupEntry, message string) error {
	return kutil.TryUpdateStatus(ctx, retry.DefaultBackoff, c, be, func() error {
		be.Status.ObservedGeneration = be.Generation
		be.Status.LastOperation = &gardencorev1beta1.LastOperation{
			Type:           gardencorev1beta1helper.ComputeOperationType(be.ObjectMeta, be.Status.LastOperation),
			State:          gardencorev1beta1.LastOperationStatePending,
			Progress:       1,
			Description:    message,
			LastUpdateTime: metav1.Now(),
		}
		return nil
	})
}

func computeGracePeriod(deletionGracePeriodHours int) time.Duration {
	return time.Hour * time.Duration(deletionGracePeriodHours)
}
