// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	"github.com/gardener/gardener/pkg/features"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/gardener/tokenrequest"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

const (
	labelKeyOrigin     = "origin"
	labelValueOperator = "gardener-operator"
)

// Reconciler reconciles Gardens.
type Reconciler struct {
	Manager               manager.Manager
	RuntimeClientSet      kubernetes.Interface
	RuntimeVersion        *semver.Version
	Config                operatorconfigv1alpha1.OperatorConfiguration
	Clock                 clock.Clock
	Recorder              record.EventRecorder
	Identity              *gardencorev1beta1.Gardener
	ComponentImageVectors imagevector.ComponentImageVectors
	GardenNamespace       string
	// GardenClientMap is the ClientMap used to communicate with the virtual garden cluster. It should be set by AddToManager function but the field is still public for usage in tests.
	GardenClientMap clientmap.ClientMap
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	garden := &operatorv1alpha1.Garden{}
	if err := r.RuntimeClientSet.Client().Get(ctx, request.NamespacedName, garden); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if err := r.ensureAtMostOneGardenExists(ctx); err != nil {
		log.Error(err, "Reconciliation prevented without automatic requeue")
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1.LastOperationTypeReconcile
	if garden.DeletionTimestamp != nil {
		operationType = gardencorev1beta1.LastOperationTypeDelete
	}

	if err := r.updateStatusOperationStart(ctx, garden, operationType); err != nil {
		return reconcile.Result{}, r.updateStatusOperationError(ctx, garden, err, operationType)
	}

	targetVersion, err := semver.NewVersion(garden.Spec.VirtualCluster.Kubernetes.Version)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed parsing version %q for virtual cluster: %w", garden.Spec.VirtualCluster.Kubernetes.Version, err)
	}

	secretsManager, err := secretsmanager.New(
		ctx,
		log.WithName("secretsmanager"),
		r.Clock,
		r.RuntimeClientSet.Client(),
		r.GardenNamespace,
		operatorv1alpha1.SecretManagerIdentityOperator,
		secretsmanager.Config{
			CASecretAutoRotation: true,
			SecretNamesToTimes:   lastSecretRotationStartTimes(garden),
		},
	)
	if err != nil {
		return reconcile.Result{}, r.updateStatusOperationError(ctx, garden, err, operationType)
	}

	if garden.DeletionTimestamp != nil {
		if result, err := r.delete(ctx, log, garden, secretsManager, targetVersion); err != nil {
			return result, r.updateStatusOperationError(ctx, garden, err, operationType)
		}
		return reconcile.Result{}, nil
	}

	if result, err := r.reconcile(ctx, log, garden, secretsManager, targetVersion); err != nil {
		return result, r.updateStatusOperationError(ctx, garden, err, operationType)
	} else if result.Requeue {
		return result, nil
	}

	return reconcile.Result{RequeueAfter: r.Config.Controllers.Garden.SyncPeriod.Duration}, r.updateStatusOperationSuccess(ctx, garden, operationType)
}

func (r *Reconciler) ensureAtMostOneGardenExists(ctx context.Context) error {
	gardenList := &metav1.PartialObjectMetadataList{}
	gardenList.SetGroupVersionKind(operatorv1alpha1.SchemeGroupVersion.WithKind("GardenList"))
	if err := r.RuntimeClientSet.Client().List(ctx, gardenList, client.Limit(2)); err != nil {
		return err
	}

	if len(gardenList.Items) <= 1 {
		return nil
	}

	return fmt.Errorf("there can be at most one operator.gardener.cloud/v1alpha1.Garden resource in the system at a time")
}

func (r *Reconciler) reportProgress(log logr.Logger, garden *operatorv1alpha1.Garden) flow.ProgressReporter {
	return flow.NewDelayingProgressReporter(clock.RealClock{}, func(ctx context.Context, stats *flow.Stats) {
		patch := client.MergeFrom(garden.DeepCopy())

		if garden.Status.LastOperation == nil {
			garden.Status.LastOperation = &gardencorev1beta1.LastOperation{}
		}
		garden.Status.LastOperation.Description = flow.MakeDescription(stats)
		garden.Status.LastOperation.Progress = stats.ProgressPercent()
		garden.Status.LastOperation.LastUpdateTime = metav1.NewTime(r.Clock.Now().UTC())

		if err := r.RuntimeClientSet.Client().Status().Patch(ctx, garden, patch); err != nil {
			log.Error(err, "Could not report reconciliation progress")
		}
	}, 5*time.Second)
}

func (r *Reconciler) updateStatusOperationStart(ctx context.Context, garden *operatorv1alpha1.Garden, operationType gardencorev1beta1.LastOperationType) error {
	var (
		now                           = metav1.NewTime(r.Clock.Now().UTC())
		description                   string
		mustRemoveOperationAnnotation bool
	)

	switch operationType {
	case gardencorev1beta1.LastOperationTypeReconcile:
		description = "Reconciliation of Garden cluster initialized."
	case gardencorev1beta1.LastOperationTypeDelete:
		description = "Deletion of Garden cluster in progress."
	}

	garden.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           operationType,
		State:          gardencorev1beta1.LastOperationStateProcessing,
		Progress:       0,
		Description:    description,
		LastUpdateTime: now,
	}
	garden.Status.Gardener = r.Identity
	garden.Status.ObservedGeneration = garden.Generation

	switch garden.Annotations[v1beta1constants.GardenerOperation] {
	case v1beta1constants.GardenerOperationReconcile:
		mustRemoveOperationAnnotation = true

	case v1beta1constants.OperationRotateCredentialsStart:
		mustRemoveOperationAnnotation = true
		startRotationCA(garden, &now)
		startRotationServiceAccountKey(garden, &now)
		startRotationETCDEncryptionKey(garden, &now)
		startRotationObservability(garden, &now)
		startRotationWorkloadIdentityKey(garden, &now)
	case v1beta1constants.OperationRotateCredentialsComplete:
		mustRemoveOperationAnnotation = true
		completeRotationCA(garden, &now)
		completeRotationServiceAccountKey(garden, &now)
		completeRotationETCDEncryptionKey(garden, &now)
		completeRotationWorkloadIdentityKey(garden, &now)

	case v1beta1constants.OperationRotateCAStart:
		mustRemoveOperationAnnotation = true
		startRotationCA(garden, &now)
	case v1beta1constants.OperationRotateCAComplete:
		mustRemoveOperationAnnotation = true
		completeRotationCA(garden, &now)

	case v1beta1constants.OperationRotateServiceAccountKeyStart:
		mustRemoveOperationAnnotation = true
		startRotationServiceAccountKey(garden, &now)
	case v1beta1constants.OperationRotateServiceAccountKeyComplete:
		mustRemoveOperationAnnotation = true
		completeRotationServiceAccountKey(garden, &now)

	case v1beta1constants.OperationRotateETCDEncryptionKeyStart:
		mustRemoveOperationAnnotation = true
		startRotationETCDEncryptionKey(garden, &now)
	case v1beta1constants.OperationRotateETCDEncryptionKeyComplete:
		mustRemoveOperationAnnotation = true
		completeRotationETCDEncryptionKey(garden, &now)

	case v1beta1constants.OperationRotateObservabilityCredentials:
		mustRemoveOperationAnnotation = true
		startRotationObservability(garden, &now)

	case operatorv1alpha1.OperationRotateWorkloadIdentityKeyStart:
		mustRemoveOperationAnnotation = true
		startRotationWorkloadIdentityKey(garden, &now)
	case operatorv1alpha1.OperationRotateWorkloadIdentityKeyComplete:
		mustRemoveOperationAnnotation = true
		completeRotationWorkloadIdentityKey(garden, &now)
	}

	if err := r.RuntimeClientSet.Client().Status().Update(ctx, garden); err != nil {
		return err
	}

	if mustRemoveOperationAnnotation {
		patch := client.MergeFrom(garden.DeepCopy())
		delete(garden.Annotations, v1beta1constants.GardenerOperation)
		return r.RuntimeClientSet.Client().Patch(ctx, garden, patch)
	}

	return nil
}

func (r *Reconciler) updateStatusOperationSuccess(ctx context.Context, garden *operatorv1alpha1.Garden, operationType gardencorev1beta1.LastOperationType) error {
	var (
		now         = metav1.NewTime(r.Clock.Now().UTC())
		description string
	)

	switch operationType {
	case gardencorev1beta1.LastOperationTypeReconcile:
		description = "Garden cluster has been successfully reconciled."
	case gardencorev1beta1.LastOperationTypeDelete:
		description = "Garden cluster has been successfully deleted."
	}

	garden.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           operationType,
		State:          gardencorev1beta1.LastOperationStateSucceeded,
		Progress:       100,
		Description:    description,
		LastUpdateTime: now,
	}

	switch helper.GetCARotationPhase(garden.Status.Credentials) {
	case gardencorev1beta1.RotationPreparing:
		helper.MutateCARotation(garden, func(rotation *gardencorev1beta1.CARotation) {
			rotation.Phase = gardencorev1beta1.RotationPrepared
			rotation.LastInitiationFinishedTime = &now
		})

	case gardencorev1beta1.RotationCompleting:
		helper.MutateCARotation(garden, func(rotation *gardencorev1beta1.CARotation) {
			rotation.Phase = gardencorev1beta1.RotationCompleted
			rotation.LastCompletionTime = &now
			rotation.LastInitiationFinishedTime = nil
			rotation.LastCompletionTriggeredTime = nil
		})
	}

	switch helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials) {
	case gardencorev1beta1.RotationPreparing:
		helper.MutateServiceAccountKeyRotation(garden, func(rotation *gardencorev1beta1.ServiceAccountKeyRotation) {
			rotation.Phase = gardencorev1beta1.RotationPrepared
			rotation.LastInitiationFinishedTime = &now
		})

	case gardencorev1beta1.RotationCompleting:
		helper.MutateServiceAccountKeyRotation(garden, func(rotation *gardencorev1beta1.ServiceAccountKeyRotation) {
			rotation.Phase = gardencorev1beta1.RotationCompleted
			rotation.LastCompletionTime = &now
			rotation.LastInitiationFinishedTime = nil
			rotation.LastCompletionTriggeredTime = nil
		})
	}

	switch helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials) {
	case gardencorev1beta1.RotationPreparing:
		helper.MutateETCDEncryptionKeyRotation(garden, func(rotation *gardencorev1beta1.ETCDEncryptionKeyRotation) {
			rotation.Phase = gardencorev1beta1.RotationPrepared
			rotation.LastInitiationFinishedTime = &now
		})

	case gardencorev1beta1.RotationCompleting:
		helper.MutateETCDEncryptionKeyRotation(garden, func(rotation *gardencorev1beta1.ETCDEncryptionKeyRotation) {
			rotation.Phase = gardencorev1beta1.RotationCompleted
			rotation.LastCompletionTime = &now
			rotation.LastInitiationFinishedTime = nil
			rotation.LastCompletionTriggeredTime = nil
		})
	}

	if helper.IsObservabilityRotationInitiationTimeAfterLastCompletionTime(garden.Status.Credentials) {
		helper.MutateObservabilityRotation(garden, func(rotation *gardencorev1beta1.ObservabilityRotation) {
			rotation.LastCompletionTime = &now
		})
	}

	switch helper.GetWorkloadIdentityKeyRotationPhase(garden.Status.Credentials) {
	case gardencorev1beta1.RotationPreparing:
		helper.MutateWorkloadIdentityKeyRotation(garden, func(rotation *operatorv1alpha1.WorkloadIdentityKeyRotation) {
			rotation.Phase = gardencorev1beta1.RotationPrepared
			rotation.LastInitiationFinishedTime = &now
		})

	case gardencorev1beta1.RotationCompleting:
		helper.MutateWorkloadIdentityKeyRotation(garden, func(rotation *operatorv1alpha1.WorkloadIdentityKeyRotation) {
			rotation.Phase = gardencorev1beta1.RotationCompleted
			rotation.LastCompletionTime = &now
			rotation.LastInitiationFinishedTime = nil
			rotation.LastCompletionTriggeredTime = nil
		})
	}

	return r.RuntimeClientSet.Client().Status().Update(ctx, garden)
}

func (r *Reconciler) updateStatusOperationError(ctx context.Context, garden *operatorv1alpha1.Garden, err error, operationType gardencorev1beta1.LastOperationType) error {
	patch := client.MergeFrom(garden.DeepCopy())

	garden.Status.Gardener = r.Identity
	if garden.Status.LastOperation == nil {
		garden.Status.LastOperation = &gardencorev1beta1.LastOperation{}
	}
	garden.Status.LastOperation.Type = operationType
	garden.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
	garden.Status.LastOperation.Description = err.Error() + " Operation will be retried."
	garden.Status.LastOperation.LastUpdateTime = metav1.NewTime(r.Clock.Now().UTC())

	if err2 := r.RuntimeClientSet.Client().Status().Patch(ctx, garden, patch); err2 != nil {
		return fmt.Errorf("failed updating last operation to state 'Error' (due to %s): %w", err.Error(), err2)
	}

	return err
}

func (r *Reconciler) generateGenericTokenKubeconfig(ctx context.Context, garden *operatorv1alpha1.Garden, secretsManager secretsmanager.Interface) error {
	genericTokenKubeconfigSecret, err := tokenrequest.GenerateGenericTokenKubeconfig(ctx, secretsManager, r.GardenNamespace, namePrefix+v1beta1constants.DeploymentNameKubeAPIServer)
	if err != nil {
		return err
	}

	if secretName := garden.Annotations[v1beta1constants.AnnotationKeyGenericTokenKubeconfigSecretName]; secretName != genericTokenKubeconfigSecret.Name {
		patch := client.MergeFrom(garden.DeepCopy())
		metav1.SetMetaDataAnnotation(&garden.ObjectMeta, v1beta1constants.AnnotationKeyGenericTokenKubeconfigSecretName, genericTokenKubeconfigSecret.Name)
		return r.RuntimeClientSet.Client().Patch(ctx, garden, patch)
	}
	return nil
}

func (r *Reconciler) cleanupGenericTokenKubeconfig(ctx context.Context, secretsManager secretsmanager.Interface) error {
	secret, exists := secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !exists {
		return nil
	}
	return client.IgnoreNotFound(r.RuntimeClientSet.Client().Delete(ctx, secret))
}

func (r *Reconciler) generateObservabilityIngressPassword(ctx context.Context, secretsManager secretsmanager.Interface) error {
	_, err := secretsManager.Generate(ctx, &secretsutils.BasicAuthSecretConfig{
		Name:           v1beta1constants.SecretNameObservabilityIngress,
		Format:         secretsutils.BasicAuthFormatNormal,
		Username:       "admin",
		PasswordLength: 32,
	}, secretsmanager.Persist(), secretsmanager.Rotate(secretsmanager.InPlace))
	return err
}

func startRotationCA(garden *operatorv1alpha1.Garden, now *metav1.Time) {
	helper.MutateCARotation(garden, func(rotation *gardencorev1beta1.CARotation) {
		rotation.Phase = gardencorev1beta1.RotationPreparing
		rotation.LastInitiationTime = now
		rotation.LastInitiationFinishedTime = nil
		rotation.LastCompletionTriggeredTime = nil
	})
}

func completeRotationCA(garden *operatorv1alpha1.Garden, now *metav1.Time) {
	helper.MutateCARotation(garden, func(rotation *gardencorev1beta1.CARotation) {
		rotation.Phase = gardencorev1beta1.RotationCompleting
		rotation.LastCompletionTriggeredTime = now
	})
}

func startRotationServiceAccountKey(garden *operatorv1alpha1.Garden, now *metav1.Time) {
	helper.MutateServiceAccountKeyRotation(garden, func(rotation *gardencorev1beta1.ServiceAccountKeyRotation) {
		rotation.Phase = gardencorev1beta1.RotationPreparing
		rotation.LastInitiationTime = now
		rotation.LastInitiationFinishedTime = nil
		rotation.LastCompletionTriggeredTime = nil
	})
}

func completeRotationServiceAccountKey(garden *operatorv1alpha1.Garden, now *metav1.Time) {
	helper.MutateServiceAccountKeyRotation(garden, func(rotation *gardencorev1beta1.ServiceAccountKeyRotation) {
		rotation.Phase = gardencorev1beta1.RotationCompleting
		rotation.LastCompletionTriggeredTime = now
	})
}

func startRotationETCDEncryptionKey(garden *operatorv1alpha1.Garden, now *metav1.Time) {
	helper.MutateETCDEncryptionKeyRotation(garden, func(rotation *gardencorev1beta1.ETCDEncryptionKeyRotation) {
		rotation.Phase = gardencorev1beta1.RotationPreparing
		rotation.LastInitiationTime = now
		rotation.LastInitiationFinishedTime = nil
		rotation.LastCompletionTriggeredTime = nil
	})
}

func completeRotationETCDEncryptionKey(garden *operatorv1alpha1.Garden, now *metav1.Time) {
	helper.MutateETCDEncryptionKeyRotation(garden, func(rotation *gardencorev1beta1.ETCDEncryptionKeyRotation) {
		rotation.Phase = gardencorev1beta1.RotationCompleting
		rotation.LastCompletionTriggeredTime = now
	})
}

func startRotationObservability(garden *operatorv1alpha1.Garden, now *metav1.Time) {
	helper.MutateObservabilityRotation(garden, func(rotation *gardencorev1beta1.ObservabilityRotation) {
		rotation.LastInitiationTime = now
	})
}

func startRotationWorkloadIdentityKey(garden *operatorv1alpha1.Garden, now *metav1.Time) {
	helper.MutateWorkloadIdentityKeyRotation(garden, func(rotation *operatorv1alpha1.WorkloadIdentityKeyRotation) {
		rotation.Phase = gardencorev1beta1.RotationPreparing
		rotation.LastInitiationTime = now
		rotation.LastInitiationFinishedTime = nil
		rotation.LastCompletionTriggeredTime = nil
	})
}

func completeRotationWorkloadIdentityKey(garden *operatorv1alpha1.Garden, now *metav1.Time) {
	helper.MutateWorkloadIdentityKeyRotation(garden, func(rotation *operatorv1alpha1.WorkloadIdentityKeyRotation) {
		rotation.Phase = gardencorev1beta1.RotationCompleting
		rotation.LastCompletionTriggeredTime = now
	})
}

func caCertConfigurations() []secretsutils.ConfigInterface {
	return append([]secretsutils.ConfigInterface{
		&secretsutils.CertificateSecretConfig{Name: operatorv1alpha1.SecretNameCARuntime, CertType: secretsutils.CACert, Validity: ptr.To(30 * 24 * time.Hour)},
	}, nonAutoRotatedCACertConfigurations()...)
}

func nonAutoRotatedCACertConfigurations() []secretsutils.ConfigInterface {
	return []secretsutils.ConfigInterface{
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAETCD, CommonName: "etcd", CertType: secretsutils.CACert},
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAETCDPeer, CommonName: "etcd-peer", CertType: secretsutils.CACert},
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCACluster, CommonName: "kubernetes", CertType: secretsutils.CACert},
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAClient, CommonName: "kubernetes-client", CertType: secretsutils.CACert},
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAFrontProxy, CommonName: "front-proxy", CertType: secretsutils.CACert},
		&secretsutils.CertificateSecretConfig{Name: operatorv1alpha1.SecretNameCAGardener, CommonName: "gardener", CertType: secretsutils.CACert},
	}
}

func caCertGenerateOptionsFor(name string, rotationPhase gardencorev1beta1.CredentialsRotationPhase) []secretsmanager.GenerateOption {
	options := []secretsmanager.GenerateOption{secretsmanager.Rotate(secretsmanager.KeepOld)}

	if name == operatorv1alpha1.SecretNameCARuntime {
		options = append(options, secretsmanager.IgnoreOldSecretsAfter(24*time.Hour))
	} else if rotationPhase == gardencorev1beta1.RotationCompleting {
		options = append(options, secretsmanager.IgnoreOldSecrets())
	}

	return options
}

func lastSecretRotationStartTimes(garden *operatorv1alpha1.Garden) map[string]time.Time {
	rotation := make(map[string]time.Time)

	if gardenStatus := garden.Status; gardenStatus.Credentials != nil && gardenStatus.Credentials.Rotation != nil {
		if gardenStatus.Credentials.Rotation.CertificateAuthorities != nil && gardenStatus.Credentials.Rotation.CertificateAuthorities.LastInitiationTime != nil {
			for _, config := range nonAutoRotatedCACertConfigurations() {
				rotation[config.GetName()] = gardenStatus.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time
			}
			rotation[kubeapiserver.SecretStaticTokenName] = gardenStatus.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time
		}

		if gardenStatus.Credentials.Rotation.ServiceAccountKey != nil && gardenStatus.Credentials.Rotation.ServiceAccountKey.LastInitiationTime != nil {
			rotation[v1beta1constants.SecretNameServiceAccountKey] = gardenStatus.Credentials.Rotation.ServiceAccountKey.LastInitiationTime.Time
		}

		if gardenStatus.Credentials.Rotation.ETCDEncryptionKey != nil && gardenStatus.Credentials.Rotation.ETCDEncryptionKey.LastInitiationTime != nil {
			rotation[v1beta1constants.SecretNameETCDEncryptionKey] = gardenStatus.Credentials.Rotation.ETCDEncryptionKey.LastInitiationTime.Time
			rotation[v1beta1constants.SecretNameGardenerETCDEncryptionKey] = gardenStatus.Credentials.Rotation.ETCDEncryptionKey.LastInitiationTime.Time
		}

		if gardenStatus.Credentials.Rotation.Observability != nil && gardenStatus.Credentials.Rotation.Observability.LastInitiationTime != nil {
			rotation[v1beta1constants.SecretNameObservabilityIngress] = gardenStatus.Credentials.Rotation.Observability.LastInitiationTime.Time
		}

		if gardenStatus.Credentials.Rotation.WorkloadIdentityKey != nil && gardenStatus.Credentials.Rotation.WorkloadIdentityKey.LastInitiationTime != nil {
			rotation[operatorv1alpha1.SecretNameWorkloadIdentityKey] = gardenStatus.Credentials.Rotation.WorkloadIdentityKey.LastInitiationTime.Time
		}
	}

	return rotation
}

func vpaEnabled(settings *operatorv1alpha1.Settings) bool {
	if settings != nil && settings.VerticalPodAutoscaler != nil {
		return ptr.Deref(settings.VerticalPodAutoscaler.Enabled, false)
	}
	return false
}

func getValidVolumeSize(volume *operatorv1alpha1.Volume, size string) string {
	if volume == nil || volume.MinimumSize == nil {
		return size
	}

	quantity, err := resource.ParseQuantity(size)
	if err == nil && quantity.Cmp(*volume.MinimumSize) < 0 {
		return volume.MinimumSize.String()
	}

	return size
}

func isIstioTLSTerminationEnabled(garden *operatorv1alpha1.Garden) bool {
	if !features.DefaultFeatureGate.Enabled(features.IstioTLSTermination) {
		return false
	}

	kubernetesVersion, err := semver.NewVersion(garden.Spec.VirtualCluster.Kubernetes.Version)
	if err != nil {
		return false
	}

	return versionutils.ConstraintK8sGreaterEqual131.Check(kubernetesVersion)
}
