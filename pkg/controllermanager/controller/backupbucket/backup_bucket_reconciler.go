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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllerutils "github.com/gardener/gardener/pkg/controllermanager/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// reconciler implements the reconcile.Reconcile interface for backupBucket reconciliation.
type reconciler struct {
	ctx      context.Context
	client   client.Client
	recorder record.EventRecorder
	logger   *logrus.Logger
}

// newReconciler returns the new backupBucker reconciler.
func newReconciler(ctx context.Context, gardenClient client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &reconciler{
		ctx:      ctx,
		client:   gardenClient,
		recorder: recorder,
		logger:   logger.Logger,
	}
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	bb := &gardencorev1alpha1.BackupBucket{}
	if err := r.client.Get(r.ctx, request.NamespacedName, bb); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Debugf("[BACKUPBUCKET RECONCILE] %s - skipping because BackupBucket has been deleted", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("[BACKUPBUCKET RECONCILE] %s - unable to retrieve object from store: %v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	if bb.Spec.Seed == nil {
		message := "Cannot reconcile BackupBucket: Waiting for BackupBucket to get scheduled on a Seed"
		r.recorder.Event(bb, corev1.EventTypeWarning, "OperationPending", message)
		return reconcile.Result{}, utilerrors.WithSuppressed(fmt.Errorf("backupBucket %s has not yet been scheduled on a Seed", bb.Name), r.updateBackupBucketStatusPending(bb, message))
	}

	if bb.DeletionTimestamp != nil {
		return r.deleteBackupBucket(bb)
	}
	// When a BackupBucket deletion timestamp is not set we need to create/reconcile the backup bucket.
	return r.reconcileBackupBucket(bb)
}

func (r *reconciler) reconcileBackupBucket(backupBucket *gardencorev1alpha1.BackupBucket) (reconcile.Result, error) {
	backupBucketLogger := logger.NewFieldLogger(logger.Logger, "backupbucket", fmt.Sprintf("%s", backupBucket.Name))

	if err := controllerutils.EnsureFinalizer(r.ctx, r.client, backupBucket, gardenv1beta1.GardenerName); err != nil {
		backupBucketLogger.Errorf("Failed to ensure gardener finalizer on backupbucket: %+v", err)
		return reconcile.Result{}, err
	}

	if updateErr := r.updateBackupBucketStatusProcessing(backupBucket, "Reconciliation of Backup Bucket state in progress.", 2); updateErr != nil {
		backupBucketLogger.Errorf("Could not update the BackupBucket status after reconciliation start: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	secret, err := common.GetSecretFromSecretRef(r.ctx, r.client, &backupBucket.Spec.SecretRef)
	if err != nil {
		backupBucketLogger.Errorf("Failed to get referred secret: %+v", err)
		return reconcile.Result{}, err
	}

	if err := controllerutils.EnsureFinalizer(r.ctx, r.client, secret, gardenv1beta1.ExternalGardenerName); err != nil {
		backupBucketLogger.Errorf("Failed to ensure external gardener finalizer on referred secret: %+v", err)
		return reconcile.Result{}, err
	}

	seedClient, err := getSeedClient(r.ctx, r.client, *backupBucket.Spec.Seed)
	if err != nil {
		return reconcile.Result{}, err
	}

	a := newActuator(r.client, seedClient, backupBucket, r.logger)
	if err := a.Reconcile(r.ctx); err != nil {
		backupBucketLogger.Errorf("Failed to reconcile backup bucket: %+v", err)

		reconcileErr := &gardencorev1alpha1.LastError{
			Codes:       gardencorev1alpha1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}
		r.recorder.Eventf(backupBucket, corev1.EventTypeWarning, gardenv1beta1.EventReconcileError, "%s", reconcileErr.Description)

		if updateErr := r.updateBackupBucketStatusError(backupBucket, reconcileErr.Description+" Operation will be retried.", reconcileErr); updateErr != nil {
			backupBucketLogger.Errorf("Could not update the BackupBucket status after deletion error: %+v", updateErr)
			return reconcile.Result{}, updateErr
		}
		return reconcile.Result{}, errors.New(reconcileErr.Description)
	}

	if updateErr := r.updateBackupBucketStatusSucceeded(backupBucket, "Backup Bucket has been successfully reconciled."); updateErr != nil {
		backupBucketLogger.Errorf("Could not update the Shoot status after reconciliation success: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	if updateErr := controllerutils.RemoveGardenerOperationAnnotation(r.ctx, retry.DefaultBackoff, r.client, backupBucket); updateErr != nil {
		backupBucketLogger.Errorf("Could not remove %q annotation: %+v", gardencorev1alpha1.GardenerOperation, updateErr)
		return reconcile.Result{}, updateErr
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) deleteBackupBucket(backupBucket *gardencorev1alpha1.BackupBucket) (reconcile.Result, error) {
	backupBucketLogger := logger.NewFieldLogger(r.logger, "backupbucket", fmt.Sprintf("%s", backupBucket.Name))
	if !sets.NewString(backupBucket.Finalizers...).Has(gardenv1beta1.GardenerName) {
		backupBucketLogger.Debug("Do not need to do anything as the BackupBucket does not have my finalizer")
		return reconcile.Result{}, nil
	}

	if updateErr := r.updateBackupBucketStatusProcessing(backupBucket, "Deletion of Backup Bucket in progress.", 2); updateErr != nil {
		backupBucketLogger.Errorf("Could not update the BackupBucket status after deletion start: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	associatedBackupEntries := make([]string, 0)
	backupEntryList := &gardencorev1alpha1.BackupEntryList{}
	if err := r.client.List(r.ctx, backupEntryList); err != nil {
		backupBucketLogger.Errorf("Could not list the backup entries associated with backupbucket: %s", err)
		return reconcile.Result{}, err
	}

	for _, entry := range backupEntryList.Items {
		if entry.Spec.BucketName == backupBucket.Name {
			associatedBackupEntries = append(associatedBackupEntries, fmt.Sprintf("%s/%s", entry.Namespace, entry.Name))
		}
	}

	if len(associatedBackupEntries) != 0 {
		message := "Can't delete BackupBucket, because BackupEntries are still referencing it."
		message += fmt.Sprintf(" BackupEntries: %+v", associatedBackupEntries)
		backupBucketLogger.Info(message)
		return reconcile.Result{}, fmt.Errorf("BackupBucket %s still has references", backupBucket.Name)
	}

	backupBucketLogger.Infof("No BackupEntries are referencing the BackupBucket. Deletion accepted.")

	seedClient, err := getSeedClient(r.ctx, r.client, *backupBucket.Spec.Seed)
	if err != nil {
		return reconcile.Result{}, err
	}
	a := newActuator(r.client, seedClient, backupBucket, r.logger)
	if err := a.Delete(r.ctx); err != nil {
		backupBucketLogger.Errorf("Failed to delete backup bucket: %+v", err)

		deleteErr := &gardencorev1alpha1.LastError{
			Codes:       gardencorev1alpha1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}
		r.recorder.Eventf(backupBucket, corev1.EventTypeWarning, gardenv1beta1.EventDeleteError, "%s", deleteErr.Description)

		if updateErr := r.updateBackupBucketStatusError(backupBucket, deleteErr.Description+" Operation will be retried.", deleteErr); updateErr != nil {
			backupBucketLogger.Errorf("Could not update the BackupBucket status after deletion error: %+v", updateErr)
			return reconcile.Result{}, updateErr
		}
		return reconcile.Result{}, errors.New(deleteErr.Description)
	}
	if updateErr := r.updateBackupBucketStatusSucceeded(backupBucket, "Backup Bucket has been successfully deleted."); updateErr != nil {
		backupBucketLogger.Errorf("Could not update the BackupBucket status after deletion successful: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	backupBucketLogger.Infof("Successfully deleted backup bucket %q", backupBucket.Name)

	secret, err := common.GetSecretFromSecretRef(r.ctx, r.client, &backupBucket.Spec.SecretRef)
	if err != nil {
		backupBucketLogger.Errorf("Failed to get referred secret: %+v", err)
		return reconcile.Result{}, err
	}

	if err := controllerutils.RemoveFinalizer(r.ctx, r.client, secret, gardenv1beta1.ExternalGardenerName); err != nil {
		backupBucketLogger.Errorf("Failed to ensure external gardener finalizer on referred secret: %+v", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, controllerutils.RemoveGardenerFinalizer(r.ctx, r.client, backupBucket)
}

func getSeedClient(ctx context.Context, gardenClient client.Client, seedName string) (client.Client, error) {
	seed := &gardenv1beta1.Seed{}
	if err := gardenClient.Get(ctx, kutil.Key(seedName), seed); err != nil {
		return nil, err
	}

	seedSecret, err := common.GetSecretFromSecretRef(ctx, gardenClient, &seed.Spec.SecretRef)
	if err != nil {
		return nil, err
	}

	kclient, err := kubernetes.NewClientFromSecretObject(seedSecret, kubernetes.WithClientOptions(
		client.Options{
			Scheme: kubernetes.SeedScheme,
		}))
	if err != nil {
		return nil, err
	}
	return kclient.Client(), err
}

func (r *reconciler) updateBackupBucketStatusProcessing(bb *gardencorev1alpha1.BackupBucket, message string, progress int) error {
	return kutil.TryUpdateStatus(r.ctx, retry.DefaultRetry, r.client, bb, func() error {
		bb.Status.LastOperation = &gardencorev1alpha1.LastOperation{
			Type:           gardencorev1alpha1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation),
			State:          gardencorev1alpha1.LastOperationStateProcessing,
			Progress:       progress,
			Description:    message,
			LastUpdateTime: metav1.Now(),
		}
		return nil
	})
}

func (r *reconciler) updateBackupBucketStatusError(bb *gardencorev1alpha1.BackupBucket, message string, lastError *gardencorev1alpha1.LastError) error {
	return kutil.TryUpdateStatus(r.ctx, retry.DefaultRetry, r.client, bb, func() error {
		progress := 1
		if bb.Status.LastOperation != nil {
			progress = bb.Status.LastOperation.Progress
		}
		bb.Status.LastOperation = &gardencorev1alpha1.LastOperation{
			Type:           gardencorev1alpha1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation),
			State:          gardencorev1alpha1.LastOperationStateError,
			Progress:       progress,
			Description:    message,
			LastUpdateTime: metav1.Now(),
		}
		bb.Status.LastError = lastError
		return nil
	})
}

func (r *reconciler) updateBackupBucketStatusPending(bb *gardencorev1alpha1.BackupBucket, message string) error {
	return kutil.TryUpdateStatus(r.ctx, retry.DefaultRetry, r.client, bb, func() error {
		progress := 1
		if bb.Status.LastOperation != nil {
			progress = bb.Status.LastOperation.Progress
		}
		bb.Status.LastOperation = &gardencorev1alpha1.LastOperation{
			Type:           gardencorev1alpha1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation),
			State:          gardencorev1alpha1.LastOperationStatePending,
			Progress:       progress,
			Description:    message,
			LastUpdateTime: metav1.Now(),
		}
		bb.Status.ObservedGeneration = bb.Generation
		return nil
	})
}

func (r *reconciler) updateBackupBucketStatusSucceeded(bb *gardencorev1alpha1.BackupBucket, message string) error {
	return kutil.TryUpdateStatus(r.ctx, retry.DefaultRetry, r.client, bb, func() error {
		bb.Status.LastOperation = &gardencorev1alpha1.LastOperation{
			Type:           gardencorev1alpha1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation),
			State:          gardencorev1alpha1.LastOperationStateSucceeded,
			Progress:       100,
			Description:    message,
			LastUpdateTime: metav1.Now(),
		}
		bb.Status.ObservedGeneration = bb.Generation
		return nil
	})
}
