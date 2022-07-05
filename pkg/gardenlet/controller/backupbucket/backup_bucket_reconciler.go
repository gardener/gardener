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
	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
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

// reconciler implements the reconcile.Reconcile interface for backupBucket reconciliation.
type reconciler struct {
	clientMap clientmap.ClientMap
	recorder  record.EventRecorder
	config    *config.GardenletConfiguration
}

// newReconciler returns the new backupBucket reconciler.
func newReconciler(clientMap clientmap.ClientMap, recorder record.EventRecorder, config *config.GardenletConfiguration) reconcile.Reconciler {
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

	bb := &gardencorev1beta1.BackupBucket{}
	if err := gardenClient.Client().Get(ctx, request.NamespacedName, bb); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if bb.Spec.SeedName == nil {
		message := "Cannot reconcile BackupBucket: Waiting for BackupBucket to get scheduled on a Seed"
		r.recorder.Event(bb, corev1.EventTypeWarning, "OperationPending", message)
		return reconcile.Result{}, utilerrors.WithSuppressed(fmt.Errorf("backupBucket %s has not yet been scheduled on a Seed", bb.Name), updateBackupBucketStatusPending(ctx, gardenClient.Client(), bb, message))
	}

	if bb.DeletionTimestamp != nil {
		return r.deleteBackupBucket(ctx, log, gardenClient, bb)
	}
	// When a BackupBucket deletion timestamp is not set we need to create/reconcile the backup bucket.
	return r.reconcileBackupBucket(ctx, log, gardenClient, bb)
}

func (r *reconciler) reconcileBackupBucket(
	ctx context.Context,
	log logr.Logger,
	gardenClient kubernetes.Interface,
	backupBucket *gardencorev1beta1.BackupBucket,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(backupBucket, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, gardenClient.Client(), backupBucket, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	if updateErr := updateBackupBucketStatusProcessing(ctx, gardenClient.Client(), backupBucket, "Reconciliation of Backup Bucket state in progress.", 2); updateErr != nil {
		return reconcile.Result{}, updateErr
	}

	secret, err := kutil.GetSecretByReference(ctx, gardenClient.Client(), &backupBucket.Spec.SecretRef)
	if err != nil {
		log.Error(err, "Failed to get backup secret", "secret", client.ObjectKey{Namespace: backupBucket.Spec.SecretRef.Namespace, Name: backupBucket.Spec.SecretRef.Name})
		r.recorder.Eventf(backupBucket, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "Failed to get backup secret %s/%s: %w", backupBucket.Spec.SecretRef.Namespace, backupBucket.Spec.SecretRef.Name, err)
		return reconcile.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(secret, gardencorev1beta1.ExternalGardenerName) {
		log.Info("Adding finalizer to referred secret")
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, gardenClient.Client(), secret, gardencorev1beta1.ExternalGardenerName); err != nil {
			log.Error(err, "Could not add finalizer to referenced secret", "secret", client.ObjectKeyFromObject(secret))
			return reconcile.Result{}, err
		}
	}

	seedClient, err := r.clientMap.GetClient(ctx, keys.ForSeedWithName(*backupBucket.Spec.SeedName))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get seed client: %w", err)
	}

	a := newActuator(log, gardenClient, seedClient, backupBucket)
	if err := a.Reconcile(ctx); err != nil {
		reconcileErr := &gardencorev1beta1.LastError{
			Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}
		r.recorder.Eventf(backupBucket, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", reconcileErr.Description)

		if updateErr := updateBackupBucketStatusError(ctx, gardenClient.Client(), backupBucket, reconcileErr.Description+" Operation will be retried.", reconcileErr); updateErr != nil {
			return reconcile.Result{}, updateErr
		}
		return reconcile.Result{}, errors.New(reconcileErr.Description)
	}

	if updateErr := updateBackupBucketStatusSucceeded(ctx, gardenClient.Client(), backupBucket, "Backup Bucket has been successfully reconciled."); updateErr != nil {
		return reconcile.Result{}, updateErr
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) deleteBackupBucket(
	ctx context.Context,
	log logr.Logger,
	gardenClient kubernetes.Interface,
	backupBucket *gardencorev1beta1.BackupBucket,
) (
	reconcile.Result,
	error,
) {
	if !sets.NewString(backupBucket.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		return reconcile.Result{}, nil
	}

	if updateErr := updateBackupBucketStatusProcessing(ctx, gardenClient.Client(), backupBucket, "Deletion of Backup Bucket in progress.", 2); updateErr != nil {
		return reconcile.Result{}, updateErr
	}

	backupEntryList := &gardencorev1beta1.BackupEntryList{}
	if err := gardenClient.Client().List(ctx, backupEntryList); err != nil {
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
		log.Info("Cannot delete because BackupEntries are still referencing the bucket", "backupEntryNames", associatedBackupEntries)
		r.recorder.Eventf(backupBucket, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, "cannot delete BackupBucket because the following BackupEntries are still referencing it: %+v", associatedBackupEntries)
		return reconcile.Result{}, fmt.Errorf("BackupBucket %s still has references", backupBucket.Name)
	}

	log.Info("No BackupEntries are referencing this BackupBucket, accepting deletion")

	seedClient, err := r.clientMap.GetClient(ctx, keys.ForSeedWithName(*backupBucket.Spec.SeedName))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get seed client: %w", err)
	}

	a := newActuator(log, gardenClient, seedClient, backupBucket)
	if err := a.Delete(ctx); err != nil {
		deleteErr := &gardencorev1beta1.LastError{
			Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}
		r.recorder.Eventf(backupBucket, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, "%s", deleteErr.Description)

		if updateErr := updateBackupBucketStatusError(ctx, gardenClient.Client(), backupBucket, deleteErr.Description+" Operation will be retried.", deleteErr); updateErr != nil {
			return reconcile.Result{}, updateErr
		}
		return reconcile.Result{}, errors.New(deleteErr.Description)
	}
	if updateErr := updateBackupBucketStatusSucceeded(ctx, gardenClient.Client(), backupBucket, "Backup Bucket has been successfully deleted."); updateErr != nil {
		return reconcile.Result{}, updateErr
	}

	log.Info("Successfully deleted")

	secret, err := kutil.GetSecretByReference(ctx, gardenClient.Client(), &backupBucket.Spec.SecretRef)
	if err != nil {
		log.Error(err, "Failed to get backup secret", "secret", client.ObjectKey{Namespace: backupBucket.Spec.SecretRef.Namespace, Name: backupBucket.Spec.SecretRef.Name})
		return reconcile.Result{}, err
	}

	log.Info("Removing finalizer on referred secret")
	if err := controllerutils.PatchRemoveFinalizers(ctx, gardenClient.Client(), secret, gardencorev1beta1.ExternalGardenerName); err != nil {
		log.Error(err, "Failed to remove external gardener finalizer on referred secret", "secret", secret)
		return reconcile.Result{}, err
	}

	log.Info("Removing finalizer")
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
