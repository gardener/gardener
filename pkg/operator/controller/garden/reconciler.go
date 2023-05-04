// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garden

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const finalizerName = "gardener.cloud/operator"

// Reconciler reconciles Gardens.
type Reconciler struct {
	RuntimeClientSet      kubernetes.Interface
	RuntimeVersion        *semver.Version
	Config                config.OperatorConfiguration
	Clock                 clock.Clock
	Recorder              record.EventRecorder
	Identity              *gardencorev1beta1.Gardener
	ImageVector           imagevector.ImageVector
	ComponentImageVectors imagevector.ComponentImageVectors
	GardenNamespace       string
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

	conditionReconciled := v1beta1helper.GetOrInitConditionWithClock(r.Clock, garden.Status.Conditions, operatorv1alpha1.GardenReconciled)
	if err := r.updateStatusOperationStart(ctx, garden, conditionReconciled); err != nil {
		return reconcile.Result{}, r.patchConditionToFalse(ctx, log, garden, conditionReconciled, err)
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
		return reconcile.Result{}, r.patchConditionToFalse(ctx, log, garden, conditionReconciled, err)
	}

	if garden.DeletionTimestamp != nil {
		if result, err := r.delete(ctx, log, garden, secretsManager, targetVersion); err != nil {
			return result, r.patchConditionToFalse(ctx, log, garden, conditionReconciled, err)
		}
		return reconcile.Result{}, nil
	}

	if result, err := r.reconcile(ctx, log, garden, secretsManager, targetVersion); err != nil {
		return result, r.patchConditionToFalse(ctx, log, garden, conditionReconciled, err)
	}

	return reconcile.Result{RequeueAfter: r.Config.Controllers.Garden.SyncPeriod.Duration}, r.updateStatusOperationSuccess(ctx, garden, conditionReconciled)
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

func (r *Reconciler) patchConditions(ctx context.Context, garden *operatorv1alpha1.Garden, condition gardencorev1beta1.Condition) error {
	patch := client.MergeFromWithOptions(garden.DeepCopy(), client.MergeFromWithOptimisticLock{})
	garden.Status.Conditions = v1beta1helper.MergeConditions(garden.Status.Conditions, condition)
	return r.RuntimeClientSet.Client().Status().Patch(ctx, garden, patch)
}

func (r *Reconciler) patchConditionToFalse(ctx context.Context, log logr.Logger, garden *operatorv1alpha1.Garden, condition gardencorev1beta1.Condition, err error) error {
	if patchErr := r.patchConditions(ctx, garden, v1beta1helper.UpdatedConditionWithClock(r.Clock, condition, gardencorev1beta1.ConditionFalse, conditionReasonPrefix(garden)+"Failed", err.Error())); patchErr != nil {
		log.Error(patchErr, "Could not patch status", "condition", condition)
	}
	return err
}

func (r *Reconciler) reportProgress(log logr.Logger, garden *operatorv1alpha1.Garden) flow.ProgressReporter {
	return flow.NewImmediateProgressReporter(func(ctx context.Context, stats *flow.Stats) {
		patch := client.MergeFrom(garden.DeepCopy())
		conditionReconciled := v1beta1helper.GetOrInitConditionWithClock(r.Clock, garden.Status.Conditions, operatorv1alpha1.GardenReconciled)
		garden.Status.Conditions = v1beta1helper.MergeConditions(garden.Status.Conditions, v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionReconciled, gardencorev1beta1.ConditionProgressing, conditionReasonPrefix(garden)+"Progressing", fmt.Sprintf("Garden operation is currently being processed (%s (%d%%)).", strings.Join(stats.Running.StringList(), ", "), stats.ProgressPercent())))
		if err := r.RuntimeClientSet.Client().Status().Patch(ctx, garden, patch); err != nil {
			log.Error(err, "Could not report reconciliation progress")
		}
	})
}

func (r *Reconciler) updateStatusOperationStart(ctx context.Context, garden *operatorv1alpha1.Garden, conditionReconciled gardencorev1beta1.Condition) error {
	garden.Status.Conditions = v1beta1helper.MergeConditions(garden.Status.Conditions, v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionReconciled, gardencorev1beta1.ConditionProgressing, conditionReasonPrefix(garden)+"Progressing", "Garden operation is currently being processed."))
	garden.Status.Gardener = r.Identity
	garden.Status.ObservedGeneration = garden.Generation

	var (
		now                           = metav1.NewTime(r.Clock.Now().UTC())
		mustRemoveOperationAnnotation bool
	)

	switch garden.Annotations[v1beta1constants.GardenerOperation] {
	case v1beta1constants.GardenerOperationReconcile:
		mustRemoveOperationAnnotation = true

	case v1beta1constants.OperationRotateCredentialsStart:
		mustRemoveOperationAnnotation = true
		startRotationCA(garden, &now)
		startRotationServiceAccountKey(garden, &now)
		startRotationETCDEncryptionKey(garden, &now)
	case v1beta1constants.OperationRotateCredentialsComplete:
		mustRemoveOperationAnnotation = true
		completeRotationCA(garden, &now)
		completeRotationServiceAccountKey(garden, &now)
		completeRotationETCDEncryptionKey(garden, &now)

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

func (r *Reconciler) updateStatusOperationSuccess(ctx context.Context, garden *operatorv1alpha1.Garden, conditionReconciled gardencorev1beta1.Condition) error {
	garden.Status.Conditions = v1beta1helper.MergeConditions(garden.Status.Conditions, v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionReconciled, gardencorev1beta1.ConditionTrue, conditionReasonPrefix(garden)+"Successful", "Garden operation was completed successfully."))

	now := metav1.NewTime(r.Clock.Now().UTC())

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

	return r.RuntimeClientSet.Client().Status().Update(ctx, garden)
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

func caCertConfigurations() []secretsutils.ConfigInterface {
	return append([]secretsutils.ConfigInterface{
		&secretsutils.CertificateSecretConfig{Name: operatorv1alpha1.SecretNameCARuntime, CertType: secretsutils.CACert, Validity: pointer.Duration(30 * 24 * time.Hour)},
	}, nonAutoRotatedCACertConfigurations()...)
}

func nonAutoRotatedCACertConfigurations() []secretsutils.ConfigInterface {
	return []secretsutils.ConfigInterface{
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAETCD, CommonName: "etcd", CertType: secretsutils.CACert},
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAETCDPeer, CommonName: "etcd-peer", CertType: secretsutils.CACert},
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCACluster, CommonName: "kubernetes", CertType: secretsutils.CACert},
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAClient, CommonName: "kubernetes-client", CertType: secretsutils.CACert},
		&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCAFrontProxy, CommonName: "front-proxy", CertType: secretsutils.CACert},
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
		}

		if gardenStatus.Credentials.Rotation.CertificateAuthorities != nil && gardenStatus.Credentials.Rotation.CertificateAuthorities.LastInitiationTime != nil {
			rotation[kubeapiserver.SecretStaticTokenName] = gardenStatus.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time
		}

		if gardenStatus.Credentials.Rotation.ServiceAccountKey != nil && gardenStatus.Credentials.Rotation.ServiceAccountKey.LastInitiationTime != nil {
			rotation[v1beta1constants.SecretNameServiceAccountKey] = gardenStatus.Credentials.Rotation.ServiceAccountKey.LastInitiationTime.Time
		}

		if gardenStatus.Credentials.Rotation.ETCDEncryptionKey != nil && gardenStatus.Credentials.Rotation.ETCDEncryptionKey.LastInitiationTime != nil {
			rotation[v1beta1constants.SecretNameETCDEncryptionKey] = gardenStatus.Credentials.Rotation.ETCDEncryptionKey.LastInitiationTime.Time
		}
	}

	return rotation
}

func conditionReasonPrefix(garden *operatorv1alpha1.Garden) string {
	if garden.DeletionTimestamp != nil {
		return "Deletion"
	}
	return "Reconciliation"
}

func vpaEnabled(settings *operatorv1alpha1.Settings) bool {
	if settings != nil && settings.VerticalPodAutoscaler != nil {
		return pointer.BoolDeref(settings.VerticalPodAutoscaler.Enabled, false)
	}
	return false
}

func hvpaEnabled() bool {
	return features.DefaultFeatureGate.Enabled(features.HVPA)
}
