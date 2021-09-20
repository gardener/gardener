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

package backupbucket

import (
	"context"
	"errors"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// reconciler implements the reconcile.Reconcile interface for backupBucket reconciliation.
type reconciler struct {
	clientMap clientmap.ClientMap
	recorder  record.EventRecorder
	logger    *logrus.Logger
	config    *config.GardenletConfiguration
}

// newReconciler returns the new backupBucket reconciler.
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

	bb := &gardencorev1beta1.BackupBucket{}
	if err := gardenClient.Client().Get(ctx, request.NamespacedName, bb); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Debugf("[BACKUPBUCKET RECONCILE] %s - skipping because BackupBucket has been deleted", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("[BACKUPBUCKET RECONCILE] %s - unable to retrieve object from store: %v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	if bb.Spec.SeedName == nil {
		message := "Cannot reconcile BackupBucket: Waiting for BackupBucket to get scheduled on a Seed"
		r.recorder.Event(bb, corev1.EventTypeWarning, "OperationPending", message)
		return reconcile.Result{}, utilerrors.WithSuppressed(fmt.Errorf("backupBucket %s has not yet been scheduled on a Seed", bb.Name), updateBackupBucketStatusPending(ctx, gardenClient.Client(), bb, message))
	}

	if bb.DeletionTimestamp != nil {
		return r.deleteBackupBucket(ctx, gardenClient, bb)
	}
	// When a BackupBucket deletion timestamp is not set we need to create/reconcile the backup bucket.
	return r.reconcileBackupBucket(ctx, gardenClient, bb)
}

func (r *reconciler) reconcileBackupBucket(ctx context.Context, gardenClient kubernetes.Interface, backupBucket *gardencorev1beta1.BackupBucket) (reconcile.Result, error) {
	backupBucketLogger := logger.NewFieldLogger(logger.Logger, "backupbucket", backupBucket.Name)

	if !controllerutil.ContainsFinalizer(backupBucket, gardencorev1beta1.GardenerName) {
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, gardenClient.Client(), backupBucket, gardencorev1beta1.GardenerName); err != nil {
			backupBucketLogger.Errorf("Failed to ensure gardener finalizer on backupbucket: %+v", err)
			return reconcile.Result{}, err
		}
	}

	if updateErr := updateBackupBucketStatusProcessing(ctx, gardenClient.Client(), backupBucket, "Reconciliation of Backup Bucket state in progress.", 2); updateErr != nil {
		backupBucketLogger.Errorf("Could not update the BackupBucket status after reconciliation start: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	secret, err := kutil.GetSecretByReference(ctx, gardenClient.Client(), &backupBucket.Spec.SecretRef)
	if err != nil {
		msg := fmt.Sprintf("Failed to get backup secret (%s/%s): %+v", backupBucket.Spec.SecretRef.Namespace, backupBucket.Spec.SecretRef.Name, err)
		backupBucketLogger.Error(msg)
		r.recorder.Eventf(backupBucket, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", msg)
		return reconcile.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(secret, gardencorev1beta1.ExternalGardenerName) {
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, gardenClient.Client(), secret, gardencorev1beta1.ExternalGardenerName); err != nil {
			backupBucketLogger.Errorf("Failed to ensure external gardener finalizer on referred secret: %+v", err)
			return reconcile.Result{}, err
		}
	}

	seedClient, err := r.clientMap.GetClient(ctx, keys.ForSeedWithName(*backupBucket.Spec.SeedName))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get seed client: %w", err)
	}

	a := newActuator(gardenClient, seedClient, backupBucket, r.logger)
	if err := a.Reconcile(ctx); err != nil {
		backupBucketLogger.Errorf("Failed to reconcile backup bucket: %+v", err)

		reconcileErr := &gardencorev1beta1.LastError{
			Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}
		r.recorder.Eventf(backupBucket, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", reconcileErr.Description)

		if updateErr := updateBackupBucketStatusError(ctx, gardenClient.Client(), backupBucket, reconcileErr.Description+" Operation will be retried.", reconcileErr); updateErr != nil {
			backupBucketLogger.Errorf("Could not update the BackupBucket status after deletion error: %+v", updateErr)
			return reconcile.Result{}, updateErr
		}
		return reconcile.Result{}, errors.New(reconcileErr.Description)
	}

	if updateErr := updateBackupBucketStatusSucceeded(ctx, gardenClient.Client(), backupBucket, "Backup Bucket has been successfully reconciled."); updateErr != nil {
		backupBucketLogger.Errorf("Could not update the Shoot status after reconciliation success: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) deleteBackupBucket(ctx context.Context, gardenClient kubernetes.Interface, backupBucket *gardencorev1beta1.BackupBucket) (reconcile.Result, error) {
	backupBucketLogger := logger.NewFieldLogger(r.logger, "backupbucket", backupBucket.Name)
	if !sets.NewString(backupBucket.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		backupBucketLogger.Debug("Do not need to do anything as the BackupBucket does not have my finalizer")
		return reconcile.Result{}, nil
	}

	if updateErr := updateBackupBucketStatusProcessing(ctx, gardenClient.Client(), backupBucket, "Deletion of Backup Bucket in progress.", 2); updateErr != nil {
		backupBucketLogger.Errorf("Could not update the BackupBucket status after deletion start: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	backupEntryList := &gardencorev1beta1.BackupEntryList{}
	if err := gardenClient.Client().List(ctx, backupEntryList); err != nil {
		backupBucketLogger.Errorf("Could not list the backup entries associated with backupbucket: %s", err)
		return reconcile.Result{}, err
	}

	// TODO: use a field-selector for this
	associatedBackupEntries := make([]string, 0)
	for _, entry := range backupEntryList.Items {
		if entry.Spec.BucketName == backupBucket.Name {
			associatedBackupEntries = append(associatedBackupEntries, client.ObjectKeyFromObject(&entry).String())
		}
	}

	if len(associatedBackupEntries) != 0 {
		message := fmt.Sprintf("Can't delete BackupBucket, because the following BackupEntries are still referencing it: %+v", associatedBackupEntries)
		backupBucketLogger.Info(message)
		r.recorder.Event(backupBucket, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, message)

		return reconcile.Result{}, fmt.Errorf("BackupBucket %s still has references", backupBucket.Name)
	}

	backupBucketLogger.Infof("No BackupEntries are referencing the BackupBucket. Deletion accepted.")

	seedClient, err := r.clientMap.GetClient(ctx, keys.ForSeedWithName(*backupBucket.Spec.SeedName))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get seed client: %w", err)
	}

	a := newActuator(gardenClient, seedClient, backupBucket, r.logger)
	if err := a.Delete(ctx); err != nil {
		backupBucketLogger.Errorf("Failed to delete backup bucket: %+v", err)

		deleteErr := &gardencorev1beta1.LastError{
			Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}
		r.recorder.Eventf(backupBucket, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, "%s", deleteErr.Description)

		if updateErr := updateBackupBucketStatusError(ctx, gardenClient.Client(), backupBucket, deleteErr.Description+" Operation will be retried.", deleteErr); updateErr != nil {
			backupBucketLogger.Errorf("Could not update the BackupBucket status after deletion error: %+v", updateErr)
			return reconcile.Result{}, updateErr
		}
		return reconcile.Result{}, errors.New(deleteErr.Description)
	}
	if updateErr := updateBackupBucketStatusSucceeded(ctx, gardenClient.Client(), backupBucket, "Backup Bucket has been successfully deleted."); updateErr != nil {
		backupBucketLogger.Errorf("Could not update the BackupBucket status after deletion successful: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	backupBucketLogger.Infof("Successfully deleted backup bucket %q", backupBucket.Name)

	secret, err := kutil.GetSecretByReference(ctx, gardenClient.Client(), &backupBucket.Spec.SecretRef)
	if err != nil {
		backupBucketLogger.Errorf("Failed to get referred secret: %+v", err)
		return reconcile.Result{}, err
	}

	if err := controllerutils.PatchRemoveFinalizers(ctx, gardenClient.Client(), secret, gardencorev1beta1.ExternalGardenerName); err != nil {
		backupBucketLogger.Errorf("Failed to remove external gardener finalizer on referred secret: %+v", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, controllerutils.PatchRemoveFinalizers(ctx, gardenClient.Client(), backupBucket, gardencorev1beta1.GardenerName)
}

func updateBackupBucketStatusProcessing(ctx context.Context, c client.StatusClient, bb *gardencorev1beta1.BackupBucket, message string, progress int32) error {
	patch := client.MergeFrom(bb.DeepCopy())
	bb.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           gardencorev1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation),
		State:          gardencorev1beta1.LastOperationStateProcessing,
		Progress:       progress,
		Description:    message,
		LastUpdateTime: metav1.Now(),
	}
	return c.Status().Patch(ctx, bb, patch)
}

func updateBackupBucketStatusError(ctx context.Context, c client.StatusClient, bb *gardencorev1beta1.BackupBucket, message string, lastError *gardencorev1beta1.LastError) error {
	patch := client.MergeFrom(bb.DeepCopy())

	var progress int32 = 1
	if bb.Status.LastOperation != nil {
		progress = bb.Status.LastOperation.Progress
	}
	bb.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           gardencorev1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation),
		State:          gardencorev1beta1.LastOperationStateError,
		Progress:       progress,
		Description:    message,
		LastUpdateTime: metav1.Now(),
	}
	bb.Status.LastError = lastError

	return c.Status().Patch(ctx, bb, patch)
}

func updateBackupBucketStatusPending(ctx context.Context, c client.StatusClient, bb *gardencorev1beta1.BackupBucket, message string) error {
	patch := client.MergeFrom(bb.DeepCopy())

	var progress int32 = 1
	if bb.Status.LastOperation != nil {
		progress = bb.Status.LastOperation.Progress
	}
	bb.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           gardencorev1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation),
		State:          gardencorev1beta1.LastOperationStatePending,
		Progress:       progress,
		Description:    message,
		LastUpdateTime: metav1.Now(),
	}
	bb.Status.ObservedGeneration = bb.Generation

	return c.Status().Patch(ctx, bb, patch)
}

func updateBackupBucketStatusSucceeded(ctx context.Context, c client.StatusClient, bb *gardencorev1beta1.BackupBucket, message string) error {
	patch := client.MergeFrom(bb.DeepCopy())

	bb.Status.LastError = nil
	bb.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           gardencorev1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation),
		State:          gardencorev1beta1.LastOperationStateSucceeded,
		Progress:       100,
		Description:    message,
		LastUpdateTime: metav1.Now(),
	}
	bb.Status.ObservedGeneration = bb.Generation

	return c.Status().Patch(ctx, bb, patch)
}
