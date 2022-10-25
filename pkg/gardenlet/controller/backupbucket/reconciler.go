// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"reflect"
	"time"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
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

// RequeueDurationWhenResourceDeletionStillPresent is the duration used for requeueing when owned resources are still in
// the process of being deleted when deleting a BackupBucket.
var RequeueDurationWhenResourceDeletionStillPresent = 5 * time.Second

// Reconciler reconciles the BackupBuckets.
type Reconciler struct {
	GardenClient    client.Client
	SeedClient      client.Client
	Config          config.BackupBucketControllerConfiguration
	Clock           clock.Clock
	Recorder        record.EventRecorder
	GardenNamespace string
	SeedName        string

	// RateLimiter allows limiting exponential backoff for testing purposes
	RateLimiter ratelimiter.RateLimiter
}

// Reconcile reconciles the BackupBuckets and deploys extensions.gardener.cloud/v1alpha1.BackupBucket in the seed cluster.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	backupBucket := &gardencorev1beta1.BackupBucket{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, backupBucket); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	extensionBackupBucket := &extensionsv1alpha1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: backupBucket.Name,
		},
	}

	if backupBucket.DeletionTimestamp != nil {
		return r.deleteBackupBucket(ctx, log, backupBucket, extensionBackupBucket)
	}
	return r.reconcileBackupBucket(ctx, log, backupBucket, extensionBackupBucket)
}

func (r *Reconciler) reconcileBackupBucket(
	ctx context.Context,
	log logr.Logger,
	backupBucket *gardencorev1beta1.BackupBucket,
	extensionBackupBucket *extensionsv1alpha1.BackupBucket,
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

	var (
		mustReconcileExtensionBackupBucket = false
		reconciliationSuccessful           = true

		extensionSecret           = r.emptyExtensionSecret(backupBucket.Name)
		extensionBackupBucketSpec = extensionsv1alpha1.BackupBucketSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           backupBucket.Spec.Provider.Type,
				ProviderConfig: backupBucket.Spec.ProviderConfig,
			},
			Region: backupBucket.Spec.Provider.Region,
			SecretRef: corev1.SecretReference{
				Name:      extensionSecret.Name,
				Namespace: extensionSecret.Namespace,
			},
		}
	)

	if err := r.SeedClient.Get(ctx, client.ObjectKeyFromObject(extensionSecret), extensionSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
	} else {
		// if the backupBucket secret data has changed, reconcile extension backupBucket
		if !reflect.DeepEqual(extensionSecret.Data, secret.Data) {
			mustReconcileExtensionBackupBucket = true
		}
	}

	if err := r.reconcileExtensionBackupBucketSecret(ctx, backupBucket); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.SeedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		// if the extension BackupBucket doesn't exist yet, create it
		mustReconcileExtensionBackupBucket = true
	} else if !reflect.DeepEqual(extensionBackupBucket.Spec, extensionBackupBucketSpec) { /* if the extensionBackupBucketSpec has changed, reconcile it*/
		mustReconcileExtensionBackupBucket = true
	} else {
		// check for errors, and if none are present, sync generated Secret to garden
		if extensionBackupBucket.Status.LastError != nil || extensionBackupBucket.Status.LastOperation.State == gardencorev1beta1.LastOperationStateError {
			mustReconcileExtensionBackupBucket = true
			reconciliationSuccessful = false

			var lastObservedError error
			if extensionBackupBucket.Status.LastError != nil {
				lastObservedError = v1beta1helper.NewErrorWithCodes(fmt.Errorf("error during reconciliation: %s", extensionBackupBucket.Status.LastError.Description), extensionBackupBucket.Status.LastError.Codes...)
			} else {
				lastObservedError = fmt.Errorf("extension state is not Succeeded but Error")
			}

			lastObservedError = v1beta1helper.NewErrorWithCodes(lastObservedError, v1beta1helper.DeprecatedDetermineErrorCodes(lastObservedError)...)
			reconcileErr := &gardencorev1beta1.LastError{
				Codes:       v1beta1helper.ExtractErrorCodes(lastObservedError),
				Description: lastObservedError.Error(),
			}

			r.Recorder.Eventf(backupBucket, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", reconcileErr.Description)

			if updateErr := updateBackupBucketStatusError(ctx, r.GardenClient, backupBucket, reconcileErr.Description+". Operation will be retried.", reconcileErr, r.Clock); updateErr != nil {
				return reconcile.Result{}, fmt.Errorf("could not update status after reconciliation error: %w", updateErr)
			}
		} else if extensionBackupBucket.Status.LastOperation.State == gardencorev1beta1.LastOperationStateSucceeded {
			if err := r.syncGeneratedSecretToGarden(ctx, backupBucket, extensionBackupBucket); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	if mustReconcileExtensionBackupBucket {
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.SeedClient, extensionBackupBucket, func() error {
			metav1.SetMetaDataAnnotation(&extensionBackupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
			metav1.SetMetaDataAnnotation(&extensionBackupBucket.ObjectMeta, v1beta1constants.GardenerTimestamp, r.Clock.Now().UTC().String())

			extensionBackupBucket.Spec = extensionBackupBucketSpec
			return nil
		}); err != nil {
			return reconcile.Result{}, err
		}
		// return early here, the BackupBucket status will be updated by the reconciliation caused by the extension BackupBucket status update.
		return reconcile.Result{}, nil
	}

	if reconciliationSuccessful {
		if updateErr := updateBackupBucketStatusSucceeded(ctx, r.GardenClient, backupBucket, "Backup Bucket has been successfully reconciled.", r.Clock); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not update status after reconciliation success: %w", updateErr)
		}
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) deleteBackupBucket(
	ctx context.Context,
	log logr.Logger,
	backupBucket *gardencorev1beta1.BackupBucket,
	extensionBackupBucket *extensionsv1alpha1.BackupBucket,
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

	if err := r.deleteGeneratedBackupBucketSecretInGarden(ctx, log, backupBucket); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileExtensionBackupBucketSecret(ctx, backupBucket); err != nil {
		return reconcile.Result{}, err
	}

	if err := extensions.DeleteExtensionObject(ctx, r.SeedClient, extensionBackupBucket); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.SeedClient.Get(ctx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
	} else {
		if lastError := extensionBackupBucket.Status.LastError; lastError != nil {
			r.Recorder.Event(backupBucket, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, lastError.Description)

			if updateErr := updateBackupBucketStatusError(ctx, r.GardenClient, backupBucket, lastError.Description+" Operation will be retried.", lastError, r.Clock); updateErr != nil {
				return reconcile.Result{}, fmt.Errorf("could not update status after deletion error: %w", updateErr)
			}
			return reconcile.Result{}, errors.New(lastError.Description)
		}
		log.Info("Extension BackupBucket not yet deleted", "extensionBackupBucket", client.ObjectKeyFromObject(extensionBackupBucket))
		return reconcile.Result{RequeueAfter: RequeueDurationWhenResourceDeletionStillPresent}, nil
	}

	if err := client.IgnoreNotFound(r.SeedClient.Delete(ctx, r.emptyExtensionSecret(backupBucket.Name))); err != nil {
		return reconcile.Result{}, err
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
		Type:           v1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation),
		State:          gardencorev1beta1.LastOperationStateProcessing,
		Progress:       progress,
		Description:    message,
		LastUpdateTime: metav1.NewTime(clock.Now()),
	}
	return c.Status().Patch(ctx, bb, patch)
}

func updateBackupBucketStatusError(ctx context.Context, c client.StatusClient, bb *gardencorev1beta1.BackupBucket, message string, lastError *gardencorev1beta1.LastError, clock clock.Clock) error {
	patch := client.MergeFrom(bb.DeepCopy())

	bb.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           v1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation),
		State:          gardencorev1beta1.LastOperationStateError,
		Progress:       50,
		Description:    message,
		LastUpdateTime: metav1.NewTime(clock.Now()),
	}
	bb.Status.LastError = lastError

	return c.Status().Patch(ctx, bb, patch)
}

func updateBackupBucketStatusSucceeded(ctx context.Context, c client.StatusClient, bb *gardencorev1beta1.BackupBucket, message string, clock clock.Clock) error {
	patch := client.MergeFrom(bb.DeepCopy())

	bb.Status.LastError = nil
	bb.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           v1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation),
		State:          gardencorev1beta1.LastOperationStateSucceeded,
		Progress:       100,
		Description:    message,
		LastUpdateTime: metav1.NewTime(clock.Now()),
	}
	bb.Status.ObservedGeneration = bb.Generation

	return c.Status().Patch(ctx, bb, patch)
}

func (r *Reconciler) emptyExtensionSecret(backupBucketName string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateBackupBucketSecretName(backupBucketName),
			Namespace: r.GardenNamespace,
		},
	}
}

func generateBackupBucketSecretName(backupBucketName string) string {
	return fmt.Sprintf("bucket-%s", backupBucketName)
}

func generateGeneratedBackupBucketSecretName(backupBucketName string) string {
	return v1beta1constants.SecretPrefixGeneratedBackupBucket + backupBucketName
}

func (r *Reconciler) reconcileExtensionBackupBucketSecret(ctx context.Context, backupBucket *gardencorev1beta1.BackupBucket) error {
	coreSecret, err := kutil.GetSecretByReference(ctx, r.GardenClient, &backupBucket.Spec.SecretRef)
	if err != nil {
		return err
	}

	extensionSecret := r.emptyExtensionSecret(backupBucket.Name)
	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, r.SeedClient, extensionSecret, func() error {
		extensionSecret.Data = coreSecret.Data
		return nil
	})
	return err
}

func (r *Reconciler) syncGeneratedSecretToGarden(ctx context.Context, backupBucket *gardencorev1beta1.BackupBucket, extensionBackupBucket *extensionsv1alpha1.BackupBucket) error {
	var gardenGeneratedSecretRef *corev1.SecretReference

	if extensionBackupBucket.Status.GeneratedSecretRef != nil {
		seedGeneratedSecret, err := kutil.GetSecretByReference(ctx, r.SeedClient, extensionBackupBucket.Status.GeneratedSecretRef)
		if err != nil {
			return err
		}

		gardenGeneratedSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generateGeneratedBackupBucketSecretName(backupBucket.Name),
				Namespace: r.GardenNamespace,
			},
		}
		ownerRef := metav1.NewControllerRef(backupBucket, gardencorev1beta1.SchemeGroupVersion.WithKind("BackupBucket"))

		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.GardenClient, gardenGeneratedSecret, func() error {
			gardenGeneratedSecret.OwnerReferences = []metav1.OwnerReference{*ownerRef}
			controllerutil.AddFinalizer(gardenGeneratedSecret, finalizerName)
			gardenGeneratedSecret.Data = seedGeneratedSecret.DeepCopy().Data
			return nil
		}); err != nil {
			return err
		}

		gardenGeneratedSecretRef = &corev1.SecretReference{
			Name:      gardenGeneratedSecret.Name,
			Namespace: gardenGeneratedSecret.Namespace,
		}
	}

	if gardenGeneratedSecretRef != nil || extensionBackupBucket.Status.ProviderStatus != nil {
		patch := client.MergeFrom(backupBucket.DeepCopy())
		backupBucket.Status.GeneratedSecretRef = gardenGeneratedSecretRef
		backupBucket.Status.ProviderStatus = extensionBackupBucket.Status.ProviderStatus
		if err := r.GardenClient.Status().Patch(ctx, backupBucket, patch); err != nil {
			return err
		}
	}
	return nil
}

// deleteGeneratedBackupBucketSecretInGarden deletes generated secret referred by core BackupBucket resource in garden.
func (r *Reconciler) deleteGeneratedBackupBucketSecretInGarden(ctx context.Context, log logr.Logger, backupBucket *gardencorev1beta1.BackupBucket) error {
	if backupBucket.Status.GeneratedSecretRef == nil {
		return nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupBucket.Status.GeneratedSecretRef.Name,
			Namespace: backupBucket.Status.GeneratedSecretRef.Namespace,
		},
	}

	if err := r.GardenClient.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get BackupBucket generated secret '%s/%s': %w", secret.Namespace, secret.Name, err)
		}
	} else if controllerutil.ContainsFinalizer(secret, finalizerName) {
		log.Info("Removing finalizer from secret", "secret", client.ObjectKeyFromObject(secret))
		if err := controllerutils.RemoveFinalizers(ctx, r.GardenClient, secret, finalizerName); err != nil {
			return fmt.Errorf("failed to remove finalizer from secret: %w", err)
		}
	}

	return client.IgnoreNotFound(r.GardenClient.Delete(ctx, secret))
}
