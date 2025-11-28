// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"errors"
	"fmt"
	"reflect"
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
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

// finalizerName is the backupbucket controller finalizer.
const finalizerName = "core.gardener.cloud/backupbucket"

// RequeueDurationWhenResourceDeletionStillPresent is the duration used for requeuing when owned resources are still in
// the process of being deleted when deleting a BackupBucket.
var RequeueDurationWhenResourceDeletionStillPresent = 5 * time.Second

// Reconciler reconciles the BackupBuckets.
type Reconciler struct {
	GardenClient    client.Client
	SeedClient      client.Client
	Config          gardenletconfigv1alpha1.BackupBucketControllerConfiguration
	Clock           clock.Clock
	Recorder        record.EventRecorder
	GardenNamespace string
	SeedName        string

	// RateLimiter allows limiting exponential backoff for testing purposes
	RateLimiter workqueue.TypedRateLimiter[reconcile.Request]
}

// Reconcile reconciles the BackupBuckets and deploys extensions.gardener.cloud/v1alpha1.BackupBucket in the seed cluster.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	gardenCtx := ctx

	seedCtx, cancel := controllerutils.GetChildReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	backupBucket := &gardencorev1beta1.BackupBucket{}
	if err := r.GardenClient.Get(gardenCtx, request.NamespacedName, backupBucket); err != nil {
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
		return r.deleteBackupBucket(gardenCtx, seedCtx, log, backupBucket, extensionBackupBucket)
	}
	return reconcile.Result{}, r.reconcileBackupBucket(gardenCtx, seedCtx, log, backupBucket, extensionBackupBucket)
}

func (r *Reconciler) reconcileBackupBucket(
	gardenCtx context.Context,
	seedCtx context.Context,
	log logr.Logger,
	backupBucket *gardencorev1beta1.BackupBucket,
	extensionBackupBucket *extensionsv1alpha1.BackupBucket,
) error {
	if !controllerutil.ContainsFinalizer(backupBucket, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(gardenCtx, r.GardenClient, backupBucket, gardencorev1beta1.GardenerName); err != nil {
			return fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	operationType := v1beta1helper.ComputeOperationType(backupBucket.ObjectMeta, backupBucket.Status.LastOperation)
	if updateErr := r.updateBackupBucketStatusOperationStart(gardenCtx, backupBucket, operationType); updateErr != nil {
		return fmt.Errorf("could not update status after reconciliation start: %w", updateErr)
	}

	backupCredentials, err := kubernetesutils.GetCredentialsByObjectReference(gardenCtx, r.GardenClient, *backupBucket.Spec.CredentialsRef)
	if err != nil {
		log.Error(err, "Failed to get backup credentials", "credentialsRef", backupBucket.Spec.CredentialsRef)
		r.Recorder.Eventf(backupBucket, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "Failed to get backup credentials from reference %v: %w", backupBucket.Spec.CredentialsRef, err)
		return err
	}

	if !controllerutil.ContainsFinalizer(backupCredentials, finalizerName) {
		log.Info("Adding finalizer to backup credentials", "finalizer", finalizerName, "gvk", backupCredentials.GetObjectKind().GroupVersionKind().String(), "key", client.ObjectKeyFromObject(backupCredentials))
		if err := controllerutils.AddFinalizers(gardenCtx, r.GardenClient, backupCredentials, finalizerName); err != nil {
			return fmt.Errorf("failed to add finalizer to backup credentials: %w", err)
		}
	}
	// TODO(dimityrmirchev): Remove this handling in a future release
	if controllerutil.ContainsFinalizer(backupCredentials, gardencorev1beta1.ExternalGardenerName) {
		log.Info("Removing finalizer from backup credentials", "finalizer", gardencorev1beta1.ExternalGardenerName, "gvk", backupCredentials.GetObjectKind().GroupVersionKind().String(), "key", client.ObjectKeyFromObject(backupCredentials))
		if err := controllerutils.RemoveFinalizers(gardenCtx, r.GardenClient, backupCredentials, gardencorev1beta1.ExternalGardenerName); err != nil {
			return fmt.Errorf("failed to remove finalizer from backup credentials: %w", err)
		}
	}

	var (
		mustReconcileExtensionBackupBucket = false
		// we should reconcile the secret only when the data has changed, since now we depend on
		// the timestamp in the secret to reconcile the extension.
		mustReconcileExtensionSecret = false

		lastObservedError         error
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

	if err := r.SeedClient.Get(seedCtx, client.ObjectKeyFromObject(extensionSecret), extensionSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// if the extension secret doesn't exist yet, create it
		mustReconcileExtensionSecret = true
	} else {
		switch credentials := backupCredentials.(type) {
		case *corev1.Secret:
			// if the backupBucket secret data has changed, reconcile extension backupBucket and extension secret
			if !reflect.DeepEqual(extensionSecret.Data, credentials.Data) {
				mustReconcileExtensionBackupBucket = true
				mustReconcileExtensionSecret = true
			}
		case *securityv1alpha1.WorkloadIdentity:
			if secretChanged, err := workloadIdentitySecretChanged(backupBucket, extensionSecret, credentials); err != nil {
				return err
			} else if secretChanged {
				mustReconcileExtensionBackupBucket = true
				mustReconcileExtensionSecret = true
			}
		}

		// if the timestamp is not present yet (needed for existing secrets), reconcile the secret
		if _, timestampPresent := extensionSecret.Annotations[v1beta1constants.GardenerTimestamp]; !timestampPresent {
			mustReconcileExtensionSecret = true
		}
	}

	if mustReconcileExtensionSecret {
		if err := r.reconcileBackupBucketExtensionSecret(seedCtx, extensionSecret, backupCredentials, backupBucket); err != nil {
			return err
		}
	}

	secretLastUpdateTime := time.Time{}
	if lastUpdateTime, ok := extensionSecret.Annotations[v1beta1constants.GardenerTimestamp]; ok {
		if secretLastUpdateTime, err = time.Parse(time.RFC3339Nano, lastUpdateTime); err != nil {
			return err
		}
	}

	// truncate the secret timestamp because extension.Status.LastOperation.LastUpdateTime
	// is represented in time.RFC3339 format which does not include the Nano precision
	secretLastUpdateTime = secretLastUpdateTime.Truncate(time.Second)

	if err := r.SeedClient.Get(seedCtx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// if the extension BackupBucket doesn't exist yet, create it
		mustReconcileExtensionBackupBucket = true
	} else if !reflect.DeepEqual(extensionBackupBucket.Spec, extensionBackupBucketSpec) ||
		(extensionBackupBucket.Status.LastOperation != nil && extensionBackupBucket.Status.LastOperation.LastUpdateTime.Time.UTC().Before(secretLastUpdateTime)) ||
		secretLastUpdateTime.IsZero() {
		// if the spec of the extensionBackupBucket has changed, or it has not been reconciled after the last update of secret, or secret last update timestamp is zero - reconcile it
		mustReconcileExtensionBackupBucket = true
	} else if extensionBackupBucket.Status.LastOperation == nil {
		// if the extension did not record a lastOperation yet, record it as error in the backupbucket status
		lastObservedError = fmt.Errorf("extension did not record a last operation yet")
		if !metav1.HasAnnotation(extensionBackupBucket.ObjectMeta, v1beta1constants.GardenerOperation) {
			mustReconcileExtensionBackupBucket = true
		}
	} else {
		// check for errors, and if none are present and extension doesn't need reconcile, sync generated Secret to garden
		lastOperationState := extensionBackupBucket.Status.LastOperation.State
		if extensionBackupBucket.Status.LastError != nil ||
			lastOperationState == gardencorev1beta1.LastOperationStateError ||
			lastOperationState == gardencorev1beta1.LastOperationStateFailed {
			if lastOperationState == gardencorev1beta1.LastOperationStateFailed {
				mustReconcileExtensionBackupBucket = true
			}

			lastObservedError = fmt.Errorf("extension state is not Succeeded but %v", lastOperationState)
			if extensionBackupBucket.Status.LastError != nil {
				lastObservedError = v1beta1helper.NewErrorWithCodes(fmt.Errorf("error during reconciliation: %s", extensionBackupBucket.Status.LastError.Description), extensionBackupBucket.Status.LastError.Codes...)
			}
		} else if lastOperationState == gardencorev1beta1.LastOperationStateSucceeded {
			// If it has been 12 hours since the last reconciliation of the BackupBucket, reconcile it
			if r.Clock.Since(extensionBackupBucket.Status.LastOperation.LastUpdateTime.Time) > 12*time.Hour {
				mustReconcileExtensionBackupBucket = true
			} else if err := r.syncGeneratedSecretToGarden(gardenCtx, seedCtx, backupBucket, extensionBackupBucket); err != nil {
				return fmt.Errorf("failed to sync generated secret to garden: %w", err)
			}
		}
	}

	if lastObservedError != nil {
		reconcileErr := &gardencorev1beta1.LastError{
			Codes:       v1beta1helper.ExtractErrorCodes(lastObservedError),
			Description: lastObservedError.Error(),
		}

		r.Recorder.Event(backupBucket, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, reconcileErr.Description)

		if updateErr := r.updateBackupBucketStatusError(gardenCtx, backupBucket, reconcileErr.Description, reconcileErr); updateErr != nil {
			return fmt.Errorf("could not update status after reconciliation error: %w", updateErr)
		}
	}

	if mustReconcileExtensionBackupBucket {
		if _, err := controllerutils.GetAndCreateOrMergePatch(seedCtx, r.SeedClient, extensionBackupBucket, func() error {
			metav1.SetMetaDataAnnotation(&extensionBackupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
			metav1.SetMetaDataAnnotation(&extensionBackupBucket.ObjectMeta, v1beta1constants.GardenerTimestamp, r.Clock.Now().UTC().Format(time.RFC3339Nano))
			metav1.SetMetaDataAnnotation(&extensionBackupBucket.ObjectMeta, v1beta1constants.AnnotationBackupBucketGeneratedSecretNamespace, r.GardenNamespace)

			extensionBackupBucket.Spec = extensionBackupBucketSpec
			return nil
		}); err != nil {
			return err
		}
		// return early here, the BackupBucket status will be updated by the reconciliation caused by the extension BackupBucket status update.
		return nil
	}

	if extensionBackupBucket.Status.LastOperation != nil && extensionBackupBucket.Status.LastOperation.State == gardencorev1beta1.LastOperationStateSucceeded {
		if updateErr := r.updateBackupBucketStatusSucceeded(gardenCtx, backupBucket, "Backup Bucket has been successfully reconciled."); updateErr != nil {
			return fmt.Errorf("could not update status after reconciliation success: %w", updateErr)
		}
	}

	return nil
}

func (r *Reconciler) deleteBackupBucket(
	gardenCtx context.Context,
	seedCtx context.Context,
	log logr.Logger,
	backupBucket *gardencorev1beta1.BackupBucket,
	extensionBackupBucket *extensionsv1alpha1.BackupBucket,
) (
	reconcile.Result,
	error,
) {
	if !sets.New(backupBucket.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		return reconcile.Result{}, nil
	}

	operationType := v1beta1helper.ComputeOperationType(backupBucket.ObjectMeta, backupBucket.Status.LastOperation)
	if updateErr := r.updateBackupBucketStatusOperationStart(gardenCtx, backupBucket, operationType); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after deletion start: %w", updateErr)
	}

	if err := r.deleteGeneratedBackupBucketSecretInGarden(gardenCtx, log, backupBucket); err != nil {
		return reconcile.Result{}, err
	}

	backupCredentials, err := kubernetesutils.GetCredentialsByObjectReference(gardenCtx, r.GardenClient, *backupBucket.Spec.CredentialsRef)
	if err != nil {
		return reconcile.Result{}, err
	}

	extensionSecret := r.emptyExtensionSecret(backupBucket.Name)
	if err := r.reconcileBackupBucketExtensionSecret(seedCtx, extensionSecret, backupCredentials, backupBucket); err != nil {
		return reconcile.Result{}, err
	}

	if err := extensions.DeleteExtensionObject(seedCtx, r.SeedClient, extensionBackupBucket); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.SeedClient.Get(seedCtx, client.ObjectKeyFromObject(extensionBackupBucket), extensionBackupBucket); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
	} else if err == nil {
		if lastError := extensionBackupBucket.Status.LastError; lastError != nil {
			r.Recorder.Event(backupBucket, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, lastError.Description)

			if updateErr := r.updateBackupBucketStatusError(gardenCtx, backupBucket, lastError.Description+" Operation will be retried.", lastError); updateErr != nil {
				return reconcile.Result{}, fmt.Errorf("could not update status after deletion error: %w", updateErr)
			}
			return reconcile.Result{}, errors.New(lastError.Description)
		}
		log.Info("Extension BackupBucket not yet deleted", "extensionBackupBucket", client.ObjectKeyFromObject(extensionBackupBucket))
		return reconcile.Result{RequeueAfter: RequeueDurationWhenResourceDeletionStillPresent}, nil
	}

	if err := client.IgnoreNotFound(r.SeedClient.Delete(seedCtx, r.emptyExtensionSecret(backupBucket.Name))); err != nil {
		return reconcile.Result{}, err
	}

	if updateErr := r.updateBackupBucketStatusSucceeded(gardenCtx, backupBucket, "Backup Bucket has been successfully deleted."); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status after deletion success: %w", updateErr)
	}

	log.Info("Successfully deleted")

	// TODO(dimityrmirchev): Remove the handling of gardencorev1beta1.ExternalGardenerName in a future release
	if controllerutil.ContainsFinalizer(backupCredentials, gardencorev1beta1.ExternalGardenerName) || controllerutil.ContainsFinalizer(backupCredentials, finalizerName) {
		log.Info("Removing finalizers from credentials",
			"finalizers", []string{gardencorev1beta1.ExternalGardenerName, finalizerName},
			"gvk", backupCredentials.GetObjectKind().GroupVersionKind().String(),
			"credentials", client.ObjectKeyFromObject(backupCredentials),
		)
		if err := controllerutils.RemoveFinalizers(gardenCtx, r.GardenClient, backupCredentials, gardencorev1beta1.ExternalGardenerName, finalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer from credentials: %w", err)
		}
	}

	if controllerutil.ContainsFinalizer(backupBucket, gardencorev1beta1.GardenerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(gardenCtx, r.GardenClient, backupBucket, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) emptyExtensionSecret(backupBucketName string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateBackupBucketSecretName(backupBucketName),
			Namespace: r.GardenNamespace,
		},
	}
}

func (r *Reconciler) reconcileBackupBucketExtensionSecret(ctx context.Context, extensionSecret *corev1.Secret, backupCredentials client.Object, backupBucket *gardencorev1beta1.BackupBucket) error {
	now := r.Clock.Now().UTC().Format(time.RFC3339Nano)
	switch credentials := backupCredentials.(type) {
	case *corev1.Secret:
		_, err := controllerutils.GetAndCreateOrMergePatch(ctx, r.SeedClient, extensionSecret, func() error {
			extensionSecret.Annotations = map[string]string{v1beta1constants.GardenerTimestamp: now}
			extensionSecret.Labels = nil
			extensionSecret.Data = credentials.Data
			return nil
		})
		return err
	case *securityv1alpha1.WorkloadIdentity:
		return workloadidentity.Deploy(ctx, r.SeedClient, credentials, extensionSecret.Name, extensionSecret.Namespace,
			map[string]string{v1beta1constants.GardenerTimestamp: now}, nil, backupBucket,
		)
	default:
		return fmt.Errorf("unsupported credentials type GVK: %q", backupCredentials.GetObjectKind().GroupVersionKind().String())
	}
}

func (r *Reconciler) syncGeneratedSecretToGarden(gardenCtx context.Context, seedCtx context.Context, backupBucket *gardencorev1beta1.BackupBucket, extensionBackupBucket *extensionsv1alpha1.BackupBucket) error {
	if extensionBackupBucket.Status.GeneratedSecretRef != nil {
		gardenGeneratedSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generateGeneratedBackupBucketSecretName(backupBucket.Name),
				Namespace: r.GardenNamespace,
			},
		}

		// Update the BackupBucket status here before going for the CreateOrGetAndStrategicMergePatch call, so that the SeedAuthorizer
		// can add the entry for this secret in the graph. See https://github.com/gardener/gardener/issues/7705 for more details.
		patch := client.MergeFrom(backupBucket.DeepCopy())
		backupBucket.Status.GeneratedSecretRef = &corev1.SecretReference{
			Name:      gardenGeneratedSecret.Name,
			Namespace: gardenGeneratedSecret.Namespace,
		}
		if err := r.GardenClient.Status().Patch(gardenCtx, backupBucket, patch); err != nil {
			return err
		}

		seedGeneratedSecret, err := kubernetesutils.GetSecretByReference(seedCtx, r.SeedClient, extensionBackupBucket.Status.GeneratedSecretRef)
		if err != nil {
			return err
		}
		if _, err := controllerutils.CreateOrGetAndStrategicMergePatch(gardenCtx, r.GardenClient, gardenGeneratedSecret, func() error {
			gardenGeneratedSecret.OwnerReferences = []metav1.OwnerReference{
				*metav1.NewControllerRef(backupBucket, gardencorev1beta1.SchemeGroupVersion.WithKind("BackupBucket")),
			}
			controllerutil.AddFinalizer(gardenGeneratedSecret, finalizerName)
			gardenGeneratedSecret.Data = seedGeneratedSecret.DeepCopy().Data
			return nil
		}); err != nil {
			return err
		}
	}

	if extensionBackupBucket.Status.ProviderStatus != nil {
		patch := client.MergeFrom(backupBucket.DeepCopy())
		backupBucket.Status.ProviderStatus = extensionBackupBucket.Status.ProviderStatus
		if err := r.GardenClient.Status().Patch(gardenCtx, backupBucket, patch); err != nil {
			return err
		}
	}

	return nil
}

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

func (r *Reconciler) updateBackupBucketStatusOperationStart(ctx context.Context, backupBucket *gardencorev1beta1.BackupBucket, operationType gardencorev1beta1.LastOperationType) error {
	var description string

	switch operationType {
	case gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationTypeReconcile:
		description = "Reconciliation of BackupBucket state initialized."

	case gardencorev1beta1.LastOperationTypeDelete:
		description = "Deletion of BackupBucket state initialized."
	}

	patch := client.MergeFrom(backupBucket.DeepCopy())

	backupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           operationType,
		State:          gardencorev1beta1.LastOperationStateProcessing,
		Progress:       0,
		Description:    description,
		LastUpdateTime: metav1.NewTime(r.Clock.Now()),
	}
	backupBucket.Status.ObservedGeneration = backupBucket.Generation

	return r.GardenClient.Status().Patch(ctx, backupBucket, patch)
}

func (r *Reconciler) updateBackupBucketStatusSucceeded(ctx context.Context, backupBucket *gardencorev1beta1.BackupBucket, message string) error {
	patch := client.MergeFrom(backupBucket.DeepCopy())

	backupBucket.Status.LastError = nil
	backupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           v1beta1helper.ComputeOperationType(backupBucket.ObjectMeta, backupBucket.Status.LastOperation),
		State:          gardencorev1beta1.LastOperationStateSucceeded,
		Progress:       100,
		Description:    message,
		LastUpdateTime: metav1.NewTime(r.Clock.Now()),
	}
	backupBucket.Status.ObservedGeneration = backupBucket.Generation

	return r.GardenClient.Status().Patch(ctx, backupBucket, patch)
}

func (r *Reconciler) updateBackupBucketStatusError(ctx context.Context, backupBucket *gardencorev1beta1.BackupBucket, message string, lastError *gardencorev1beta1.LastError) error {
	patch := client.MergeFrom(backupBucket.DeepCopy())

	backupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           v1beta1helper.ComputeOperationType(backupBucket.ObjectMeta, backupBucket.Status.LastOperation),
		State:          gardencorev1beta1.LastOperationStateError,
		Progress:       50,
		Description:    message,
		LastUpdateTime: metav1.NewTime(r.Clock.Now()),
	}
	backupBucket.Status.LastError = lastError

	return r.GardenClient.Status().Patch(ctx, backupBucket, patch)
}

func generateBackupBucketSecretName(backupBucketName string) string {
	return fmt.Sprintf("bucket-%s", backupBucketName)
}

func generateGeneratedBackupBucketSecretName(backupBucketName string) string {
	return v1beta1constants.SecretPrefixGeneratedBackupBucket + backupBucketName
}

func workloadIdentitySecretChanged(backupBucket *gardencorev1beta1.BackupBucket, secretToCompareTo *corev1.Secret, workloadIdentity *securityv1alpha1.WorkloadIdentity) (bool, error) {
	gvk, err := apiutil.GVKForObject(backupBucket, kubernetes.GardenScheme)
	if err != nil {
		return false, err
	}
	opts := []workloadidentity.SecretOption{
		workloadidentity.For(workloadIdentity.GetName(), workloadIdentity.GetNamespace(), workloadIdentity.Spec.TargetSystem.Type),
		workloadidentity.WithProviderConfig(workloadIdentity.Spec.TargetSystem.ProviderConfig),
		workloadidentity.WithContextObject(securityv1alpha1.ContextObject{APIVersion: gvk.GroupVersion().String(), Kind: gvk.Kind, Name: backupBucket.GetName(), UID: backupBucket.GetUID()}),
	}
	if val, ok := secretToCompareTo.Annotations[v1beta1constants.GardenerTimestamp]; ok {
		opts = append(opts, workloadidentity.WithAnnotations(map[string]string{v1beta1constants.GardenerTimestamp: val}))
	}
	s, err := workloadidentity.NewSecret(secretToCompareTo.Name, secretToCompareTo.Namespace, opts...)
	if err != nil {
		return false, err
	}

	return !s.Equal(secretToCompareTo), nil
}
