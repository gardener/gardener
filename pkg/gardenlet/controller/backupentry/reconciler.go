// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsbackupentry "github.com/gardener/gardener/pkg/component/extensions/backupentry"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	// DefaultTimeout defines how long the controller should wait until the BackupBucket resource is ready or is successfully deleted. Exposed for tests.
	DefaultTimeout = 30 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by the component is treated as 'severe'. Exposed for tests.
	DefaultSevereThreshold = 15 * time.Second
	// DefaultInterval is the default interval for retry operations. Exposed for tests.
	DefaultInterval = 5 * time.Second
)

// RequeueDurationWhenResourceDeletionStillPresent is the duration used for requeuing when owned resources are still in
// the process of being deleted when deleting a BackupEntry.
var RequeueDurationWhenResourceDeletionStillPresent = 5 * time.Second

// Reconciler reconciles the BackupEntries.
type Reconciler struct {
	GardenClient    client.Client
	SeedClient      client.Client
	Recorder        record.EventRecorder
	Config          gardenletconfigv1alpha1.BackupEntryControllerConfiguration
	Clock           clock.Clock
	SeedName        string
	GardenNamespace string

	// RateLimiter allows limiting exponential backoff for testing purposes
	RateLimiter workqueue.TypedRateLimiter[reconcile.Request]
}

// Reconcile reconciles the BackupEntry and deploys extensions.gardener.cloud/v1alpha1.BackupEntry in the seed cluster.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	gardenCtx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	seedCtx, cancel := controllerutils.GetChildReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	backupEntry := &gardencorev1beta1.BackupEntry{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, backupEntry); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if responsibleSeedName := gardenerutils.GetResponsibleSeedName(backupEntry.Spec.SeedName, backupEntry.Status.SeedName); responsibleSeedName != r.SeedName {
		log.Info("Skipping because BackupEntry is not managed by this gardenlet", "seedName", responsibleSeedName)
		return reconcile.Result{}, nil
	}

	if backupEntry.DeletionTimestamp != nil {
		return r.deleteBackupEntry(gardenCtx, seedCtx, log, backupEntry)
	}

	if shouldMigrateBackupEntry(backupEntry) {
		return r.migrateBackupEntry(gardenCtx, seedCtx, log, backupEntry)
	}

	return reconcile.Result{}, r.reconcileBackupEntry(gardenCtx, seedCtx, log, backupEntry)
}

func (r *Reconciler) reconcileBackupEntry(
	gardenCtx context.Context,
	seedCtx context.Context,
	log logr.Logger,
	backupEntry *gardencorev1beta1.BackupEntry,
) error {
	if !controllerutil.ContainsFinalizer(backupEntry, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(gardenCtx, r.GardenClient, backupEntry, gardencorev1beta1.GardenerName); err != nil {
			return fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	operationType := v1beta1helper.ComputeOperationType(backupEntry.ObjectMeta, backupEntry.Status.LastOperation)
	if updateErr := r.updateBackupEntryStatusOperationStart(gardenCtx, backupEntry, operationType); updateErr != nil {
		return fmt.Errorf("could not update status after reconciliation start: %w", updateErr)
	}

	var (
		mustReconcileExtensionBackupEntry = false
		// we should reconcile the secret only when the data has changed, since now we depend on
		// the timestamp in the secret to reconcile the extension.
		mustReconcileExtensionSecret = false

		lastObservedError error
		extensionSecret   = r.emptyExtensionSecret(backupEntry)
		component         = r.newExtensionComponent(log, backupEntry)

		backupBucket = &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: backupEntry.Spec.BucketName,
			},
		}
		extensionBackupEntry = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: backupEntry.Name,
			},
		}
	)

	if err := r.checkIfBackupBucketIsHealthy(gardenCtx, backupBucket); err != nil {
		reconcileErr := &gardencorev1beta1.LastError{
			Codes:       v1beta1helper.ExtractErrorCodes(err),
			Description: err.Error(),
		}

		r.Recorder.Event(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, reconcileErr.Description)

		if updateErr := r.updateBackupEntryStatusError(gardenCtx, backupEntry, operationType, reconcileErr.Description, reconcileErr); updateErr != nil {
			return fmt.Errorf("could not update status after reconciliation error: %w", updateErr)
		}

		// the backupEntry will be requeued when the state of the BackupBucket changes to Succeeded
		return nil
	}

	gardenSecret, err := r.getGardenSecret(gardenCtx, backupBucket)
	if err != nil {
		return err
	}

	if err := r.SeedClient.Get(seedCtx, client.ObjectKeyFromObject(extensionSecret), extensionSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// if the extension secret doesn't exist yet, create it
		mustReconcileExtensionSecret = true
	} else {
		// if the backupBucket secret data has changed, reconcile extension backupEntry and extension secret
		if !reflect.DeepEqual(extensionSecret.Data, gardenSecret.Data) {
			mustReconcileExtensionBackupEntry = true
			mustReconcileExtensionSecret = true
		}
		// if the timestamp is not present yet (needed for existing secrets), reconcile the secret
		if _, timestampPresent := extensionSecret.Annotations[v1beta1constants.GardenerTimestamp]; !timestampPresent {
			mustReconcileExtensionSecret = true
		}
	}

	if mustReconcileExtensionSecret {
		if err := r.reconcileBackupEntryExtensionSecret(seedCtx, extensionSecret, gardenSecret); err != nil {
			return err
		}
	}

	extensionBackupEntrySpec := extensionsv1alpha1.BackupEntrySpec{
		DefaultSpec: extensionsv1alpha1.DefaultSpec{
			Type:           backupBucket.Spec.Provider.Type,
			ProviderConfig: backupBucket.Spec.ProviderConfig,
		},
		Region: backupBucket.Spec.Provider.Region,
		SecretRef: corev1.SecretReference{
			Name:      extensionSecret.Name,
			Namespace: extensionSecret.Namespace,
		},
		BucketName:                 backupEntry.Spec.BucketName,
		BackupBucketProviderStatus: backupBucket.Status.ProviderStatus,
	}

	secretLastUpdateTime, err := time.Parse(time.RFC3339Nano, extensionSecret.Annotations[v1beta1constants.GardenerTimestamp])
	if err != nil {
		return err
	}

	// truncate the secret timestamp because extension.Status.LastOperation.LastUpdateTime
	// is represented in time.RFC3339 format which does not include the Nano precision
	secretLastUpdateTime = secretLastUpdateTime.Truncate(time.Second)

	if err := r.SeedClient.Get(seedCtx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// if the extension BackupEntry doesn't exist yet, create it
		mustReconcileExtensionBackupEntry = true
	} else if !reflect.DeepEqual(extensionBackupEntry.Spec, extensionBackupEntrySpec) ||
		(extensionBackupEntry.Status.LastOperation != nil && extensionBackupEntry.Status.LastOperation.LastUpdateTime.Time.UTC().Before(secretLastUpdateTime)) {
		// if the spec of the extensionBackupEntry has changed or it has not been reconciled after the last updation of secret, reconcile it
		mustReconcileExtensionBackupEntry = true
	} else if extensionBackupEntry.Status.LastOperation == nil {
		// if the extension did not record a lastOperation yet, record it as error in the backupentry status
		lastObservedError = errors.New("extension did not record a last operation yet")
		if !metav1.HasAnnotation(extensionBackupEntry.ObjectMeta, v1beta1constants.GardenerOperation) {
			mustReconcileExtensionBackupEntry = true
		}
	} else {
		// check for errors, and if none are present, reconciliation has succeeded
		lastOperationState := extensionBackupEntry.Status.LastOperation.State
		if extensionBackupEntry.Status.LastError != nil ||
			lastOperationState == gardencorev1beta1.LastOperationStateError ||
			lastOperationState == gardencorev1beta1.LastOperationStateFailed {
			if lastOperationState == gardencorev1beta1.LastOperationStateFailed {
				mustReconcileExtensionBackupEntry = true
			}

			lastObservedError = fmt.Errorf("extension state is not Succeeded but %v", lastOperationState)
			if extensionBackupEntry.Status.LastError != nil {
				lastObservedError = v1beta1helper.NewErrorWithCodes(fmt.Errorf("error during reconciliation: %s", extensionBackupEntry.Status.LastError.Description), extensionBackupEntry.Status.LastError.Codes...)
			}
		}
	}

	if lastObservedError != nil {
		reconcileErr := &gardencorev1beta1.LastError{
			Codes:       v1beta1helper.ExtractErrorCodes(lastObservedError),
			Description: lastObservedError.Error(),
		}

		r.Recorder.Event(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, reconcileErr.Description)

		if updateErr := r.updateBackupEntryStatusError(gardenCtx, backupEntry, operationType, reconcileErr.Description, reconcileErr); updateErr != nil {
			return fmt.Errorf("could not update status after reconciliation error: %w", updateErr)
		}
	}

	if mustReconcileExtensionBackupEntry {
		if err := r.reconcileBackupEntryExtension(gardenCtx, seedCtx, backupBucket, backupEntry, component); err != nil {
			return err
		}
		// return early here, the BackupEntry status will be updated by the reconciliation caused by the extension BackupEntry status update.
		return nil
	}

	if extensionBackupEntry.Status.LastOperation != nil && extensionBackupEntry.Status.LastOperation.State == gardencorev1beta1.LastOperationStateSucceeded {
		if updateErr := r.updateBackupEntryStatusSucceeded(gardenCtx, backupEntry, operationType); updateErr != nil {
			return fmt.Errorf("could not update status after reconciliation success: %w", updateErr)
		}

		if kubernetesutils.HasMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRestore) {
			if updateErr := removeGardenerOperationAnnotation(gardenCtx, r.GardenClient, backupEntry); updateErr != nil {
				return fmt.Errorf("could not remove %q annotation: %w", v1beta1constants.GardenerOperation, updateErr)
			}
		}
	}

	return nil
}

func (r *Reconciler) deleteBackupEntry(
	gardenCtx context.Context,
	seedCtx context.Context,
	log logr.Logger,
	backupEntry *gardencorev1beta1.BackupEntry,
) (
	reconcile.Result,
	error,
) {
	if !sets.New(backupEntry.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		log.V(1).Info("Do not need to do anything as the BackupEntry does not have my finalizer")
		return reconcile.Result{}, nil
	}

	gracePeriod := computeGracePeriod(*r.Config.DeletionGracePeriodHours, r.Config.DeletionGracePeriodShootPurposes, gardencorev1beta1.ShootPurpose(backupEntry.Annotations[v1beta1constants.ShootPurpose]))
	present, _ := strconv.ParseBool(backupEntry.Annotations[gardencorev1beta1.BackupEntryForceDeletion])
	if present || r.Clock.Since(backupEntry.DeletionTimestamp.Local()) > gracePeriod {
		operationType := v1beta1helper.ComputeOperationType(backupEntry.ObjectMeta, backupEntry.Status.LastOperation)
		if updateErr := r.updateBackupEntryStatusOperationStart(gardenCtx, backupEntry, operationType); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not update status after deletion start: %w", updateErr)
		}

		extensionSecret := r.emptyExtensionSecret(backupEntry)
		backupBucket := &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: backupEntry.Spec.BucketName,
			},
		}

		if err := r.checkIfBackupBucketIsHealthy(gardenCtx, backupBucket); err != nil {
			reconcileErr := &gardencorev1beta1.LastError{
				Codes:       v1beta1helper.ExtractErrorCodes(err),
				Description: err.Error(),
			}

			r.Recorder.Event(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, reconcileErr.Description)

			if updateErr := r.updateBackupEntryStatusError(gardenCtx, backupEntry, operationType, reconcileErr.Description, reconcileErr); updateErr != nil {
				return reconcile.Result{}, fmt.Errorf("could not update status after reconciliation error: %w", updateErr)
			}

			// the backupEntry will be requeued when the state of the BackupBucket changes to Succeeded
			return reconcile.Result{}, nil
		}

		gardenSecret, err := r.getGardenSecret(gardenCtx, backupBucket)
		if err != nil {
			return reconcile.Result{}, err
		}

		if err := r.reconcileBackupEntryExtensionSecret(seedCtx, extensionSecret, gardenSecret); err != nil {
			return reconcile.Result{}, err
		}

		component := r.newExtensionComponent(log, backupEntry)
		if err := component.Destroy(seedCtx); err != nil {
			return reconcile.Result{}, err
		}

		extensionBackupEntry := &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: backupEntry.Name,
			},
		}

		if err := r.SeedClient.Get(seedCtx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry); err != nil {
			if !apierrors.IsNotFound(err) {
				return reconcile.Result{}, err
			}
		} else if err == nil {
			if lastError := extensionBackupEntry.Status.LastError; lastError != nil {
				r.Recorder.Event(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, lastError.Description)

				if updateErr := r.updateBackupEntryStatusError(gardenCtx, backupEntry, operationType, lastError.Description, lastError); updateErr != nil {
					return reconcile.Result{}, fmt.Errorf("could not update status after deletion error: %w", updateErr)
				}
				return reconcile.Result{}, errors.New(lastError.Description)
			}
			log.Info("Extension BackupEntry not yet deleted", "extensionBackupEntry", client.ObjectKeyFromObject(extensionBackupEntry))
			return reconcile.Result{RequeueAfter: RequeueDurationWhenResourceDeletionStillPresent}, nil
		}

		if err := client.IgnoreNotFound(r.SeedClient.Delete(seedCtx, extensionSecret)); err != nil {
			return reconcile.Result{}, err
		}

		if updateErr := r.updateBackupEntryStatusSucceeded(gardenCtx, backupEntry, operationType); updateErr != nil {
			return reconcile.Result{}, fmt.Errorf("could not update status after deletion success: %w", updateErr)
		}

		log.Info("Successfully deleted")

		if controllerutil.ContainsFinalizer(backupEntry, gardencorev1beta1.GardenerName) {
			log.Info("Removing finalizer")
			if err := controllerutils.RemoveFinalizers(gardenCtx, r.GardenClient, backupEntry, gardencorev1beta1.GardenerName); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}

		return reconcile.Result{}, nil
	}

	if updateErr := r.updateBackupEntryStatusPending(gardenCtx, backupEntry, fmt.Sprintf("Deletion of backup entry is scheduled for %s", backupEntry.DeletionTimestamp.Add(gracePeriod))); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after deletion success: %w", updateErr)
	}

	requeueAfter := backupEntry.DeletionTimestamp.Time.Add(gracePeriod).Sub(r.Clock.Now())
	if requeueAfter < 0 {
		return reconcile.Result{}, fmt.Errorf("the backupentry should have been deleted by now")
	}
	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}

func (r *Reconciler) migrateBackupEntry(
	gardenCtx context.Context,
	seedCtx context.Context,
	log logr.Logger,
	backupEntry *gardencorev1beta1.BackupEntry,
) (
	reconcile.Result,
	error,
) {
	if !sets.New(backupEntry.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		log.V(1).Info("Do not need to do anything as the BackupEntry does not have my finalizer")
		return reconcile.Result{}, nil
	}

	if updateErr := r.updateBackupEntryStatusOperationStart(gardenCtx, backupEntry, gardencorev1beta1.LastOperationTypeMigrate); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after migration start: %w", updateErr)
	}

	var (
		extensionSecret = r.emptyExtensionSecret(backupEntry)
		component       = r.newExtensionComponent(log, backupEntry)

		extensionBackupEntry = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: backupEntry.Name,
			},
		}
	)

	if err := r.SeedClient.Get(seedCtx, client.ObjectKeyFromObject(extensionBackupEntry), extensionBackupEntry); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
	} else {
		lastOperation := extensionBackupEntry.Status.LastOperation
		if lastOperation == nil {
			return reconcile.Result{}, fmt.Errorf("extension object did not record a lastOperation yet")
		}

		switch lastOperation.Type {
		case gardencorev1beta1.LastOperationTypeMigrate:
			if extensionBackupEntry.Status.LastError != nil ||
				lastOperation.State == gardencorev1beta1.LastOperationStateError ||
				lastOperation.State == gardencorev1beta1.LastOperationStateFailed {
				lastError := fmt.Errorf("extension state is not Succeeded but %v", lastOperation.State)
				if extensionBackupEntry.Status.LastError != nil {
					lastError = v1beta1helper.NewErrorWithCodes(fmt.Errorf("error during reconciliation: %s", extensionBackupEntry.Status.LastError.Description), extensionBackupEntry.Status.LastError.Codes...)
				}

				migrateError := &gardencorev1beta1.LastError{
					Codes:       v1beta1helper.ExtractErrorCodes(lastError),
					Description: lastError.Error(),
				}

				r.Recorder.Event(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, migrateError.Description)

				description := migrateError.Description
				if updateErr := r.updateBackupEntryStatusError(gardenCtx, backupEntry, gardencorev1beta1.LastOperationTypeMigrate, description, migrateError); updateErr != nil {
					return reconcile.Result{}, fmt.Errorf("could not update status after migration error: %w", updateErr)
				}

				if lastOperation.State == gardencorev1beta1.LastOperationStateFailed {
					return reconcile.Result{}, errors.New(migrateError.Description)
				}
				return reconcile.Result{}, nil
			} else if lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded {
				if err := component.Destroy(seedCtx); err != nil {
					return reconcile.Result{}, err
				}
				return reconcile.Result{RequeueAfter: RequeueDurationWhenResourceDeletionStillPresent}, nil
			}
		case gardencorev1beta1.LastOperationTypeDelete:
			if lastError := extensionBackupEntry.Status.LastError; lastError != nil {
				r.Recorder.Event(backupEntry, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, lastError.Description)

				if updateErr := r.updateBackupEntryStatusError(gardenCtx, backupEntry, gardencorev1beta1.LastOperationTypeDelete, lastError.Description, lastError); updateErr != nil {
					return reconcile.Result{}, fmt.Errorf("could not update status after deletion error: %w", updateErr)
				}
				return reconcile.Result{}, errors.New(lastError.Description)
			}
			log.Info("Extension BackupEntry not yet deleted", "extensionBackupEntry", client.ObjectKeyFromObject(extensionBackupEntry))
			return reconcile.Result{RequeueAfter: RequeueDurationWhenResourceDeletionStillPresent}, nil
		default:
			return reconcile.Result{}, component.Migrate(seedCtx)
		}
	}

	if err := client.IgnoreNotFound(r.SeedClient.Delete(seedCtx, extensionSecret)); err != nil {
		return reconcile.Result{}, err
	}

	if updateErr := r.updateBackupEntryStatusSucceeded(gardenCtx, backupEntry, gardencorev1beta1.LastOperationTypeMigrate); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after migration success: %w", updateErr)
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) updateBackupEntryStatusOperationStart(ctx context.Context, be *gardencorev1beta1.BackupEntry, operationType gardencorev1beta1.LastOperationType) error {
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
		LastUpdateTime: metav1.NewTime(r.Clock.Now()),
	}
	be.Status.ObservedGeneration = be.Generation
	if be.Status.SeedName == nil {
		be.Status.SeedName = be.Spec.SeedName
	}

	return r.GardenClient.Status().Patch(ctx, be, patch)
}

func (r *Reconciler) updateBackupEntryStatusError(ctx context.Context, be *gardencorev1beta1.BackupEntry, operationType gardencorev1beta1.LastOperationType, description string, lastError *gardencorev1beta1.LastError) error {
	patch := client.MergeFrom(be.DeepCopy())

	be.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           operationType,
		State:          gardencorev1beta1.LastOperationStateError,
		Progress:       50,
		Description:    description,
		LastUpdateTime: metav1.NewTime(r.Clock.Now()),
	}
	be.Status.LastError = lastError

	return r.GardenClient.Status().Patch(ctx, be, patch)
}

func (r *Reconciler) updateBackupEntryStatusSucceeded(ctx context.Context, be *gardencorev1beta1.BackupEntry, operationType gardencorev1beta1.LastOperationType) error {
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
		LastUpdateTime: metav1.NewTime(r.Clock.Now()),
	}
	if operationType == gardencorev1beta1.LastOperationTypeMigrate {
		be.Status.SeedName = nil
	}

	return r.GardenClient.Status().Patch(ctx, be, patch)
}

func (r *Reconciler) updateBackupEntryStatusPending(ctx context.Context, be *gardencorev1beta1.BackupEntry, message string) error {
	patch := client.MergeFrom(be.DeepCopy())

	be.Status.ObservedGeneration = be.Generation
	be.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           v1beta1helper.ComputeOperationType(be.ObjectMeta, be.Status.LastOperation),
		State:          gardencorev1beta1.LastOperationStatePending,
		Progress:       0,
		Description:    message,
		LastUpdateTime: metav1.NewTime(r.Clock.Now()),
	}

	return r.GardenClient.Status().Patch(ctx, be, patch)
}

func (r *Reconciler) emptyExtensionSecret(backupEntry *gardencorev1beta1.BackupEntry) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "entry-" + backupEntry.Name,
			Namespace: r.GardenNamespace,
		},
	}
}

func (r *Reconciler) newExtensionComponent(log logr.Logger, backupEntry *gardencorev1beta1.BackupEntry) extensionsbackupentry.Interface {
	extensionSecret := r.emptyExtensionSecret(backupEntry)
	return extensionsbackupentry.New(
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
		extensionsbackupentry.DefaultInterval,
		extensionsbackupentry.DefaultSevereThreshold,
		extensionsbackupentry.DefaultTimeout,
	)
}

func (r *Reconciler) checkIfBackupBucketIsHealthy(ctx context.Context, backupBucket *gardencorev1beta1.BackupBucket) error {
	if err := r.GardenClient.Get(ctx, client.ObjectKeyFromObject(backupBucket), backupBucket); err != nil {
		return fmt.Errorf("failed getting associated BackupBucket %q: %w", backupBucket.Name, err)
	}

	if backupBucket.Status.LastError != nil {
		return v1beta1helper.NewErrorWithCodes(fmt.Errorf("error during reconciliation of associated BackupBucket: %s", backupBucket.Status.LastError.Description), backupBucket.Status.LastError.Codes...)
	}

	if backupBucket.Status.ObservedGeneration != backupBucket.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", backupBucket.Status.ObservedGeneration, backupBucket.Generation)
	}

	if backupBucket.Status.LastOperation == nil {
		return fmt.Errorf("associated BackupBucket did not record a last operation yet")
	}

	if backupBucket.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
		return fmt.Errorf("associated BackupBucket state is not Succeeded but %v", backupBucket.Status.LastOperation.State)
	}

	return nil
}

func (r *Reconciler) getGardenSecret(ctx context.Context, backupBucket *gardencorev1beta1.BackupBucket) (*corev1.Secret, error) {
	gardenSecretRef := &backupBucket.Spec.SecretRef
	if backupBucket.Status.GeneratedSecretRef != nil {
		gardenSecretRef = backupBucket.Status.GeneratedSecretRef
	}

	gardenSecret, err := kubernetesutils.GetSecretByReference(ctx, r.GardenClient, gardenSecretRef)
	if err != nil {
		return nil, fmt.Errorf("could not get secret referred in core backup bucket: %w", err)
	}

	return gardenSecret, nil
}

func (r *Reconciler) reconcileBackupEntryExtensionSecret(ctx context.Context, extensionSecret, gardenSecret *corev1.Secret) error {
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.SeedClient, extensionSecret, func() error {
		metav1.SetMetaDataAnnotation(&extensionSecret.ObjectMeta, v1beta1constants.GardenerTimestamp, r.Clock.Now().UTC().Format(time.RFC3339Nano))
		extensionSecret.Data = gardenSecret.DeepCopy().Data
		return nil
	}); err != nil {
		return fmt.Errorf("could not reconcile extension secret in seed: %w", err)
	}

	return nil
}

// reconcileBackupEntryExtension deploys the BackupEntry extension resource in Seed with the required secret.
func (r *Reconciler) reconcileBackupEntryExtension(gardenCtx context.Context, seedCtx context.Context, backupBucket *gardencorev1beta1.BackupBucket, backupEntry *gardencorev1beta1.BackupEntry, component extensionsbackupentry.Interface) error {
	component.SetType(backupBucket.Spec.Provider.Type)
	component.SetProviderConfig(backupBucket.Spec.ProviderConfig)
	component.SetRegion(backupBucket.Spec.Provider.Region)
	component.SetBackupBucketProviderStatus(backupBucket.Status.ProviderStatus)

	if !isRestorePhase(backupEntry) {
		return component.Deploy(seedCtx)
	}

	shootName := gardenerutils.GetShootNameFromOwnerReferences(backupEntry)
	shootState := &gardencorev1beta1.ShootState{}
	if err := r.GardenClient.Get(gardenCtx, client.ObjectKey{Namespace: backupEntry.Namespace, Name: shootName}, shootState); err != nil {
		return err
	}
	return component.Restore(seedCtx, shootState)
}

func shouldMigrateBackupEntry(be *gardencorev1beta1.BackupEntry) bool {
	return be.Status.SeedName != nil && be.Spec.SeedName != nil && *be.Spec.SeedName != *be.Status.SeedName
}

// isRestorePhase checks if the BackupEntry's LastOperation is Restore
func isRestorePhase(backupEntry *gardencorev1beta1.BackupEntry) bool {
	return backupEntry.Status.LastOperation != nil && backupEntry.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeRestore
}

func computeGracePeriod(deletionGracePeriodHours int, deletionGracePeriodShootPurposes []gardencorev1beta1.ShootPurpose, shootPurpose gardencorev1beta1.ShootPurpose) time.Duration {
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
