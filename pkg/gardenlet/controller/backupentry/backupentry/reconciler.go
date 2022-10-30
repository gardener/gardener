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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	extensionsbackupentry "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/backupentry"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var (
	// DefaultTimeout defines how long the controller should wait until the extension resource is ready or is succesfully deleted. Exposed for tests.
	DefaultTimeout = 30 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by the component is treated as 'severe'. Exposed for tests.
	DefaultSevereThreshold = 15 * time.Second
	// DefaultInterval is the default interval for retry operations. Exposed for tests.
	DefaultInterval = 5 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait
	// for a successful reconciliation of a BackupEntry resource.
	ExtensionsDefaultTimeout = extensionsbackupentry.DefaultTimeout
	// DefaultInterval is the default interval for retry operations.
	ExtensionsDefaultInterval = extensionsbackupentry.DefaultInterval
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as 'severe'.
	ExtensionsDefaultSevereThreshold = extensionsbackupentry.DefaultSevereThreshold
)

// Reconciler reconciles the BackupEntries.
type Reconciler struct {
	GardenClient    client.Client
	SeedClient      client.Client
	Recorder        record.EventRecorder
	Config          config.BackupEntryControllerConfiguration
	Clock           clock.Clock
	SeedName        string
	GardenNamespace string

	// RateLimiter allows limiting exponential backoff for testing purposes
	RateLimiter ratelimiter.RateLimiter
}

// Reconcile reconciles the BackupEntry and deploys extensions.gardener.cloud/v1alpha1.BackupEnrry in the seed cluster.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	backupEntry := &gardencorev1beta1.BackupEntry{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, backupEntry); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// Remove the operation annotation if its value is not "restore"
	// If it's "restore", it will be removed at the end of the reconciliation since it's needed
	// to properly determine that the operation is "restore, and not "reconcile"
	if operationType, ok := backupEntry.Annotations[v1beta1constants.GardenerOperation]; ok && operationType != v1beta1constants.GardenerOperationRestore {
		if updateErr := removeGardenerOperationAnnotation(ctx, r.GardenClient, backupEntry); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not remove %q annotation: %w", v1beta1constants.GardenerOperation, updateErr)
		}
	}

	if backupEntry.DeletionTimestamp != nil {
		return r.deleteBackupEntry(ctx, log, backupEntry)
	}

	if shouldMigrateBackupEntry(backupEntry) {
		return r.migrateBackupEntry(ctx, log, backupEntry)
	}

	if !IsBackupEntryManagedByThisGardenlet(backupEntry, r.SeedName) {
		log.V(1).Info("Skipping because BackupEntry is not managed by this gardenlet", "seedName", *backupEntry.Spec.SeedName)
		return reconcile.Result{}, nil
	}

	return r.reconcileBackupEntry(ctx, log, backupEntry)
}

func (r *Reconciler) reconcileBackupEntry(
	ctx context.Context,
	log logr.Logger,
	backupEntry *gardencorev1beta1.BackupEntry,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(backupEntry, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.GardenClient, backupEntry, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(backupEntry.ObjectMeta, backupEntry.Status.LastOperation)
	if updateErr := r.updateBackupEntryStatusOperationStart(ctx, r.GardenClient, r.Clock, backupEntry, operationType); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after reconciliation start: %w", updateErr)
	}

	extensionSecret := r.emptyExtensionSecret(backupEntry)
	backupBucket := &gardencorev1beta1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: backupEntry.Spec.BucketName,
		},
	}

	component := extensionsbackupentry.New(
		log,
		r.SeedClient,
		r.Clock,
		&extensionsbackupentry.Values{
			Name:       backupEntry.Name,
			BucketName: backupEntry.Spec.BucketName,
			SecretRef: corev1.SecretReference{
				Name:      extensionSecret.Name,
				Namespace: extensionSecret.Namespace,
			},
		},
		ExtensionsDefaultInterval,
		ExtensionsDefaultSevereThreshold,
		ExtensionsDefaultTimeout,
	)

	if err := r.waitUntilBackupBucketReconciled(ctx, log, backupBucket); err != nil {
		return reconcile.Result{}, fmt.Errorf("associated BackupBucket %q is not ready yet with err: %w", backupEntry.Spec.BucketName, err)
	}

	if err := r.deployBackupEntryExtensionSecret(ctx, backupBucket, backupEntry); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.deployBackupEntryExtension(ctx, backupBucket, backupEntry, component); err != nil {
		return reconcile.Result{}, err
	}

	if err := component.Wait(ctx); err != nil {
		log.Error(err, "Failed to reconcile")

		reconcileErr := &gardencorev1beta1.LastError{
			Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}
		r.Recorder.Eventf(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", reconcileErr.Description)
		if updateErr := updateBackupEntryStatusError(ctx, r.GardenClient, r.Clock, backupEntry, operationType, reconcileErr); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not update status after reconciliation error: %w", updateErr)
		}
		return reconcile.Result{}, errors.New(reconcileErr.Description)
	}

	if updateErr := updateBackupEntryStatusSucceeded(ctx, r.GardenClient, r.Clock, backupEntry, operationType); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after reconciliation success: %w", updateErr)
	}

	if kutil.HasMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRestore) {
		if updateErr := removeGardenerOperationAnnotation(ctx, r.GardenClient, backupEntry); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not remove %q annotation: %w", v1beta1constants.GardenerOperation, updateErr)
		}
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) deleteBackupEntry(
	ctx context.Context,
	log logr.Logger,
	backupEntry *gardencorev1beta1.BackupEntry,
) (
	reconcile.Result,
	error,
) {
	if !sets.NewString(backupEntry.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		log.V(1).Info("Do not need to do anything as the BackupEntry does not have my finalizer")
		return reconcile.Result{}, nil
	}

	gracePeriod := computeGracePeriod(*r.Config.DeletionGracePeriodHours, r.Config.DeletionGracePeriodShootPurposes, gardencore.ShootPurpose(backupEntry.Annotations[v1beta1constants.ShootPurpose]))
	present, _ := strconv.ParseBool(backupEntry.ObjectMeta.Annotations[gardencorev1beta1.BackupEntryForceDeletion])
	if present || r.Clock.Since(backupEntry.DeletionTimestamp.Local()) > gracePeriod {
		operationType := gardencorev1beta1helper.ComputeOperationType(backupEntry.ObjectMeta, backupEntry.Status.LastOperation)
		if updateErr := r.updateBackupEntryStatusOperationStart(ctx, r.GardenClient, r.Clock, backupEntry, operationType); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not update status after deletion start: %w", updateErr)
		}

		extensionSecret := r.emptyExtensionSecret(backupEntry)
		backupBucket := &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: backupEntry.Spec.BucketName,
			},
		}

		if err := r.waitUntilBackupBucketReconciled(ctx, log, backupBucket); err != nil {
			return reconcile.Result{}, fmt.Errorf("associated BackupBucket %q is not ready yet with err: %w", backupEntry.Spec.BucketName, err)
		}

		if err := r.deployBackupEntryExtensionSecret(ctx, backupBucket, backupEntry); err != nil {
			return reconcile.Result{}, err
		}

		component := extensionsbackupentry.New(
			log,
			r.SeedClient,
			r.Clock,
			&extensionsbackupentry.Values{
				Name:       backupEntry.Name,
				BucketName: backupEntry.Spec.BucketName,
				SecretRef: corev1.SecretReference{
					Name:      extensionSecret.Name,
					Namespace: extensionSecret.Namespace,
				},
			},
			ExtensionsDefaultInterval,
			ExtensionsDefaultSevereThreshold,
			ExtensionsDefaultTimeout,
		)

		if err := component.Destroy(ctx); err != nil {
			return reconcile.Result{}, err
		}

		if err := component.WaitCleanup(ctx); err != nil {
			log.Error(err, "Failed to delete")

			deleteErr := &gardencorev1beta1.LastError{
				Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
				Description: err.Error(),
			}
			r.Recorder.Eventf(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, "%s", deleteErr.Description)

			if updateErr := updateBackupEntryStatusError(ctx, r.GardenClient, r.Clock, backupEntry, operationType, deleteErr); updateErr != nil {
				return reconcile.Result{}, fmt.Errorf("could not update status after deletion error: %w", updateErr)
			}
			return reconcile.Result{}, errors.New(deleteErr.Description)
		}

		if err := client.IgnoreNotFound(r.SeedClient.Delete(ctx, extensionSecret)); err != nil {
			return reconcile.Result{}, nil
		}

		if updateErr := updateBackupEntryStatusSucceeded(ctx, r.GardenClient, r.Clock, backupEntry, operationType); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not update status after deletion success: %w", updateErr)
		}

		log.Info("Successfully deleted")

		if controllerutil.ContainsFinalizer(backupEntry, gardencorev1beta1.GardenerName) {
			log.Info("Removing finalizer")
			if err := controllerutils.RemoveFinalizers(ctx, r.GardenClient, backupEntry, gardencorev1beta1.GardenerName); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}

		return reconcile.Result{}, nil
	}

	if updateErr := updateBackupEntryStatusPending(ctx, r.GardenClient, r.Clock, backupEntry, fmt.Sprintf("Deletion of backup entry is scheduled for %s", backupEntry.DeletionTimestamp.Time.Add(gracePeriod))); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after deletion success: %w", updateErr)
	}

	return reconcile.Result{}, nil
}

func shouldMigrateBackupEntry(be *gardencorev1beta1.BackupEntry) bool {
	return be.Status.SeedName != nil && be.Spec.SeedName != nil && *be.Spec.SeedName != *be.Status.SeedName
}

func (r *Reconciler) migrateBackupEntry(
	ctx context.Context,
	log logr.Logger,
	backupEntry *gardencorev1beta1.BackupEntry,
) (
	reconcile.Result,
	error,
) {
	if !sets.NewString(backupEntry.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		log.V(1).Info("Do not need to do anything as the BackupEntry does not have my finalizer")
		return reconcile.Result{}, nil
	}

	if updateErr := r.updateBackupEntryStatusOperationStart(ctx, r.GardenClient, r.Clock, backupEntry, gardencorev1beta1.LastOperationTypeMigrate); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after migration start: %w", updateErr)
	}

	extensionSecret := r.emptyExtensionSecret(backupEntry)
	component := extensionsbackupentry.New(
		log,
		r.SeedClient,
		r.Clock,
		&extensionsbackupentry.Values{
			Name:       backupEntry.Name,
			BucketName: backupEntry.Spec.BucketName,
			SecretRef: corev1.SecretReference{
				Name:      extensionSecret.Name,
				Namespace: extensionSecret.Namespace,
			},
		},
		ExtensionsDefaultInterval,
		ExtensionsDefaultSevereThreshold,
		ExtensionsDefaultTimeout,
	)

	if err := component.Migrate(ctx); err != nil {
		return reconcile.Result{}, err
	}

	if err := component.WaitMigrate(ctx); err != nil {
		log.Error(err, "Failed to migrate")

		reconcileErr := &gardencorev1beta1.LastError{
			Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}
		r.Recorder.Eventf(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", reconcileErr.Description)

		if updateErr := updateBackupEntryStatusError(ctx, r.GardenClient, r.Clock, backupEntry, gardencorev1beta1.LastOperationTypeMigrate, reconcileErr); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not update status after migration error: %w", updateErr)
		}
		return reconcile.Result{}, errors.New(reconcileErr.Description)
	}

	if err := component.Destroy(ctx); err != nil {
		return reconcile.Result{}, err
	}

	if err := component.WaitCleanup(ctx); err != nil {
		return reconcile.Result{}, err
	}

	if err := client.IgnoreNotFound(r.SeedClient.Delete(ctx, extensionSecret)); err != nil {
		return reconcile.Result{}, nil
	}

	if updateErr := updateBackupEntryStatusSucceeded(ctx, r.GardenClient, r.Clock, backupEntry, gardencorev1beta1.LastOperationTypeMigrate); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after migration success: %w", updateErr)
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) updateBackupEntryStatusOperationStart(ctx context.Context, c client.StatusClient, clock clock.Clock, be *gardencorev1beta1.BackupEntry, operationType gardencorev1beta1.LastOperationType) error {
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
		LastUpdateTime: metav1.NewTime(clock.Now()),
	}
	be.Status.ObservedGeneration = be.Generation
	if be.Status.SeedName == nil {
		be.Status.SeedName = be.Spec.SeedName
	}

	return c.Status().Patch(ctx, be, patch)
}

func updateBackupEntryStatusError(ctx context.Context, c client.StatusClient, clock clock.Clock, be *gardencorev1beta1.BackupEntry, operationType gardencorev1beta1.LastOperationType, lastError *gardencorev1beta1.LastError) error {
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
		LastUpdateTime: metav1.NewTime(clock.Now()),
	}
	be.Status.LastError = lastError

	return c.Status().Patch(ctx, be, patch)
}

func updateBackupEntryStatusSucceeded(ctx context.Context, c client.StatusClient, clock clock.Clock, be *gardencorev1beta1.BackupEntry, operationType gardencorev1beta1.LastOperationType) error {
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
		LastUpdateTime: metav1.NewTime(clock.Now()),
	}
	if operationType == gardencorev1beta1.LastOperationTypeMigrate {
		be.Status.SeedName = nil
	}

	return c.Status().Patch(ctx, be, patch)
}

func updateBackupEntryStatusPending(ctx context.Context, c client.StatusClient, clock clock.Clock, be *gardencorev1beta1.BackupEntry, message string) error {
	patch := client.MergeFrom(be.DeepCopy())

	be.Status.ObservedGeneration = be.Generation
	be.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           gardencorev1beta1helper.ComputeOperationType(be.ObjectMeta, be.Status.LastOperation),
		State:          gardencorev1beta1.LastOperationStatePending,
		Progress:       0,
		Description:    message,
		LastUpdateTime: metav1.NewTime(clock.Now()),
	}

	return c.Status().Patch(ctx, be, patch)
}

func (r *Reconciler) emptyExtensionSecret(backupEntry *gardencorev1beta1.BackupEntry) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("entry-%s", backupEntry.Name),
			Namespace: r.GardenNamespace,
		},
	}
}

func (r *Reconciler) waitUntilBackupBucketReconciled(ctx context.Context, log logr.Logger, backupBucket *gardencorev1beta1.BackupBucket) error {
	return extensions.WaitUntilObjectReadyWithHealthFunction(
		ctx,
		r.GardenClient,
		log,
		health.CheckBackupBucket,
		backupBucket,
		"BackupBucket",
		DefaultInterval,
		DefaultSevereThreshold,
		DefaultTimeout,
		nil,
	)
}

func (r *Reconciler) deployBackupEntryExtensionSecret(ctx context.Context, backupBucket *gardencorev1beta1.BackupBucket, backupEntry *gardencorev1beta1.BackupEntry) error {
	coreSecretRef := &backupBucket.Spec.SecretRef
	if backupBucket.Status.GeneratedSecretRef != nil {
		coreSecretRef = backupBucket.Status.GeneratedSecretRef
	}

	coreSecret, err := kutil.GetSecretByReference(ctx, r.GardenClient, coreSecretRef)
	if err != nil {
		return fmt.Errorf("could not get secret referred in core backup bucket: %w", err)
	}

	// create secret for extension BackupEntry in seed
	extensionSecret := r.emptyExtensionSecret(backupEntry)
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.SeedClient, extensionSecret, func() error {
		extensionSecret.Data = coreSecret.DeepCopy().Data
		return nil
	}); err != nil {
		return fmt.Errorf("could not reconcile extension secret in seed: %w", err)
	}

	return nil
}

// deployBackupEntryExtension deploys the BackupEntry extension resource in Seed with the required secret.
func (r *Reconciler) deployBackupEntryExtension(ctx context.Context, backupBucket *gardencorev1beta1.BackupBucket, backupEntry *gardencorev1beta1.BackupEntry, component extensionsbackupentry.Interface) error {
	component.SetType(backupBucket.Spec.Provider.Type)
	component.SetProviderConfig(backupBucket.Spec.ProviderConfig)
	component.SetRegion(backupBucket.Spec.Provider.Region)
	component.SetBackupBucketProviderStatus(backupBucket.Status.ProviderStatus)

	if !isRestorePhase(backupEntry) {
		return component.Deploy(ctx)
	}

	shootName := gutil.GetShootNameFromOwnerReferences(backupEntry)
	shootState := &gardencorev1alpha1.ShootState{}
	if err := r.GardenClient.Get(ctx, kutil.Key(backupEntry.Namespace, shootName), shootState); err != nil {
		return err
	}
	return component.Restore(ctx, shootState)
}

// isRestorePhase checks if the BackupEntry's LastOperation is Restore
func isRestorePhase(backupEntry *gardencorev1beta1.BackupEntry) bool {
	return backupEntry.Status.LastOperation != nil && backupEntry.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeRestore
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

// IsBackupEntryManagedByThisGardenlet checks if the given BackupEntry is managed by this gardenlet by comparing it with the seed name from the GardenletConfiguration.
func IsBackupEntryManagedByThisGardenlet(backupEntry *gardencorev1beta1.BackupEntry, seedName string) bool {
	return pointer.StringDeref(backupEntry.Spec.SeedName, "") == seedName
}
