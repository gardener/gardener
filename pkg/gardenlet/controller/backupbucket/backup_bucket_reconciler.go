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

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// finalizerName is the backupbucket controller finalizer.
const finalizerName = "core.gardener.cloud/backupbucket"

// reconciler implements the reconcile.Reconcile interface for backupBucket reconciliation.
type Reconciler struct {
	GardenClient client.Client
	SeedClient   client.Client
	Clock        clock.Clock
	Recorder     record.EventRecorder
	Config       config.BackupBucketControllerConfiguration

	// RateLimiter allows limiting exponential backoff for testing purposes
	RateLimiter ratelimiter.RateLimiter
}

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	bb := &gardencorev1beta1.BackupBucket{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, bb); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if bb.Spec.SeedName == nil {
		message := "Cannot reconcile BackupBucket: Waiting for BackupBucket to get scheduled on a Seed"
		r.Recorder.Event(bb, corev1.EventTypeWarning, "OperationPending", message)
		return reconcile.Result{}, utilerrors.WithSuppressed(fmt.Errorf("backupBucket %s has not yet been scheduled on a Seed", bb.Name), updateBackupBucketStatusPending(ctx, r.GardenClient, bb, message, r.Clock))
	}

	if bb.DeletionTimestamp != nil {
		return r.deleteBackupBucket(ctx, log, bb)
	}
	// When a BackupBucket deletion timestamp is not set we need to create/reconcile the backup bucket.
	return r.reconcileBackupBucket(ctx, log, bb)
}

func (r *Reconciler) reconcileBackupBucket(
	ctx context.Context,
	log logr.Logger,
	backupBucket *gardencorev1beta1.BackupBucket,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(backupBucket, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.GardenClient, backupBucket, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if updateErr := updateBackupBucketStatusProcessing(ctx, r.GardenClient, backupBucket, "Reconciliation of Backup Bucket state in progress.", 2, r.Clock); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after reconciliation start: %w", updateErr)
	}

	secret, err := kutil.GetSecretByReference(ctx, r.GardenClient, &backupBucket.Spec.SecretRef)
	if err != nil {
		log.Error(err, "Failed to get backup secret", "secret", client.ObjectKey{Namespace: backupBucket.Spec.SecretRef.Namespace, Name: backupBucket.Spec.SecretRef.Name})
		r.Recorder.Eventf(backupBucket, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "Failed to get backup secret %s/%s: %w", backupBucket.Spec.SecretRef.Namespace, backupBucket.Spec.SecretRef.Name, err)
		return reconcile.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(secret, gardencorev1beta1.ExternalGardenerName) {
		log.Info("Adding finalizer to secret", "secret", client.ObjectKeyFromObject(secret))
		if err := controllerutils.AddFinalizers(ctx, r.GardenClient, secret, gardencorev1beta1.ExternalGardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer to secret: %w", err)
		}
	}

	a := newActuator(log, r.GardenClient, r.SeedClient, r.Clock, backupBucket)
	if err := a.Reconcile(ctx); err != nil {
		reconcileErr := &gardencorev1beta1.LastError{
			Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}
		r.Recorder.Eventf(backupBucket, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", reconcileErr.Description)

		if updateErr := updateBackupBucketStatusError(ctx, r.GardenClient, backupBucket, reconcileErr.Description+" Operation will be retried.", reconcileErr, r.Clock); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not update status after reconciliation error: %w", updateErr)
		}
		return reconcile.Result{}, errors.New(reconcileErr.Description)
	}

	if updateErr := updateBackupBucketStatusSucceeded(ctx, r.GardenClient, backupBucket, "Backup Bucket has been successfully reconciled.", r.Clock); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after reconciliation success: %w", updateErr)
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) deleteBackupBucket(
	ctx context.Context,
	log logr.Logger,
	backupBucket *gardencorev1beta1.BackupBucket,
) (
	reconcile.Result,
	error,
) {
	if !sets.NewString(backupBucket.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		return reconcile.Result{}, nil
	}

	if updateErr := updateBackupBucketStatusProcessing(ctx, r.GardenClient, backupBucket, "Deletion of Backup Bucket in progress.", 2, r.Clock); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after deletion start: %w", updateErr)
	}

	backupEntryList := &gardencorev1beta1.BackupEntryList{}
	if err := r.GardenClient.List(ctx, backupEntryList, client.MatchingFields{core.BackupEntryBucketName: backupBucket.Name}); err != nil {
		return reconcile.Result{}, err
	}

	associatedBackupEntries := make([]string, 0)
	for _, entry := range backupEntryList.Items {
		associatedBackupEntries = append(associatedBackupEntries, client.ObjectKeyFromObject(&entry).String())
	}

	if len(associatedBackupEntries) != 0 {
		log.Info("Cannot delete because BackupEntries are still referencing the bucket", "backupEntryNames", associatedBackupEntries)
		r.Recorder.Eventf(backupBucket, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, "cannot delete BackupBucket because the following BackupEntries are still referencing it: %+v", associatedBackupEntries)
		return reconcile.Result{}, fmt.Errorf("BackupBucket %s still has references", backupBucket.Name)
	}

	log.Info("No BackupEntries are referencing this BackupBucket, accepting deletion")

	a := newActuator(log, r.GardenClient, r.SeedClient, r.Clock, backupBucket)
	if err := a.Delete(ctx); err != nil {
		deleteErr := &gardencorev1beta1.LastError{
			Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}
		r.Recorder.Eventf(backupBucket, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, "%s", deleteErr.Description)

		if updateErr := updateBackupBucketStatusError(ctx, r.GardenClient, backupBucket, deleteErr.Description+" Operation will be retried.", deleteErr, r.Clock); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not update status after deletion error: %w", updateErr)
		}
		return reconcile.Result{}, errors.New(deleteErr.Description)
	}
	if updateErr := updateBackupBucketStatusSucceeded(ctx, r.GardenClient, backupBucket, "Backup Bucket has been successfully deleted.", r.Clock); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after deletion success: %w", updateErr)
	}

	log.Info("Successfully deleted")

	secret, err := kutil.GetSecretByReference(ctx, r.GardenClient, &backupBucket.Spec.SecretRef)
	if err != nil {
		log.Error(err, "Failed to get backup secret", "secret", client.ObjectKey{Namespace: backupBucket.Spec.SecretRef.Namespace, Name: backupBucket.Spec.SecretRef.Name})
		return reconcile.Result{}, err
	}

	if controllerutil.ContainsFinalizer(secret, gardencorev1beta1.ExternalGardenerName) {
		log.Info("Removing finalizer from secret", "secret", client.ObjectKeyFromObject(secret))
		if err := controllerutils.RemoveFinalizers(ctx, r.GardenClient, secret, gardencorev1beta1.ExternalGardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer from secret: %w", err)
		}
	}

	if controllerutil.ContainsFinalizer(backupBucket, gardencorev1beta1.GardenerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.GardenClient, backupBucket, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func updateBackupBucketStatusProcessing(ctx context.Context, c client.StatusClient, bb *gardencorev1beta1.BackupBucket, message string, progress int32, clock clock.Clock) error {
	patch := client.MergeFrom(bb.DeepCopy())
	bb.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           gardencorev1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation),
		State:          gardencorev1beta1.LastOperationStateProcessing,
		Progress:       progress,
		Description:    message,
		LastUpdateTime: metav1.NewTime(clock.Now()),
	}
	return c.Status().Patch(ctx, bb, patch)
}

func updateBackupBucketStatusError(ctx context.Context, c client.StatusClient, bb *gardencorev1beta1.BackupBucket, message string, lastError *gardencorev1beta1.LastError, clock clock.Clock) error {
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
		LastUpdateTime: metav1.NewTime(clock.Now()),
	}
	bb.Status.LastError = lastError

	return c.Status().Patch(ctx, bb, patch)
}

func updateBackupBucketStatusPending(ctx context.Context, c client.StatusClient, bb *gardencorev1beta1.BackupBucket, message string, clock clock.Clock) error {
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
		LastUpdateTime: metav1.NewTime(clock.Now()),
	}
	bb.Status.ObservedGeneration = bb.Generation

	return c.Status().Patch(ctx, bb, patch)
}

func updateBackupBucketStatusSucceeded(ctx context.Context, c client.StatusClient, bb *gardencorev1beta1.BackupBucket, message string, clock clock.Clock) error {
	patch := client.MergeFrom(bb.DeepCopy())

	bb.Status.LastError = nil
	bb.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           gardencorev1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation),
		State:          gardencorev1beta1.LastOperationStateSucceeded,
		Progress:       100,
		Description:    message,
		LastUpdateTime: metav1.NewTime(clock.Now()),
	}
	bb.Status.ObservedGeneration = bb.Generation

	return c.Status().Patch(ctx, bb, patch)
}
