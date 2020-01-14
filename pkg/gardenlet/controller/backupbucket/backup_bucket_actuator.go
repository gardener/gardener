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
	"fmt"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultTimeout  = 30 * time.Second
	defaultInterval = 5 * time.Second
)

// Actuator acts upon BackupBucket resources.
type Actuator interface {
	// Reconcile reconciles the BackupBucket.
	Reconcile(context.Context) error
	// Delete deletes the BackupBucket.
	Delete(context.Context) error
}

type actuator struct {
	gardenClient client.Client
	seedClient   client.Client
	logger       logrus.FieldLogger
	backupBucket *gardencorev1beta1.BackupBucket
}

func newActuator(gardenClient, seedClient client.Client, bb *gardencorev1beta1.BackupBucket, logger logrus.FieldLogger) Actuator {
	return &actuator{
		logger:       logger.WithField("backupbucket", bb.Name),
		backupBucket: bb,
		gardenClient: gardenClient,
		seedClient:   seedClient,
	}
}

func (a *actuator) Reconcile(ctx context.Context) error {
	var (
		g = flow.NewGraph("Backup Bucket Reconciliation")

		deployBackupBucketExtension = g.Add(flow.Task{
			Name: "Deploying backup bucket extension resource",
			Fn:   flow.TaskFn(a.deployBackupBucketExtension).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})

		_ = g.Add(flow.Task{
			Name:         "Waiting until backup bucket is reconciled",
			Fn:           flow.TaskFn(a.waitUntilBackupBucketExtensionReconciled),
			Dependencies: flow.NewTaskIDs(deployBackupBucketExtension),
		})
		f = g.Compile()
	)

	return f.Run(flow.Opts{
		Logger:           a.logger,
		ProgressReporter: a.reportBackupBucketProgress,
		Context:          ctx,
	})
}

func (a *actuator) Delete(ctx context.Context) error {
	var (
		g                  = flow.NewGraph("Backup bucket deletion")
		deleteBackupBucket = g.Add(flow.Task{
			Name: "Destroying backup bucket",
			Fn:   flow.TaskFn(a.deleteBackupBucketExtension),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until extension backup bucket is deleted",
			Fn:           flow.TaskFn(a.waitUntilBackupBucketExtensionDeleted),
			Dependencies: flow.NewTaskIDs(deleteBackupBucket),
		})
		f = g.Compile()
	)
	return f.Run(flow.Opts{
		Logger:           a.logger,
		ProgressReporter: a.reportBackupBucketProgress,
		Context:          ctx,
	})
}

// reportBackupBucketProgress will update the phase and error in the BackupBucket manifest `status` section
// by the current progress of the Flow execution.
func (a *actuator) reportBackupBucketProgress(ctx context.Context, stats *flow.Stats) {
	if err := kutil.TryUpdateStatus(ctx, kretry.DefaultRetry, a.gardenClient, a.backupBucket, func() error {
		a.backupBucket.Status.LastOperation.Description = makeDescription(stats)
		a.backupBucket.Status.LastOperation.Progress = stats.ProgressPercent()
		a.backupBucket.Status.LastOperation.LastUpdateTime = metav1.Now()
		return nil
	}); err != nil {
		a.logger.Warnf("could not report backupbucket progress with description: %s", makeDescription(stats))
	}
}

func makeDescription(stats *flow.Stats) string {
	if stats.ProgressPercent() == 100 {
		return "Execution finished"
	}
	return strings.Join(stats.Running.StringList(), ", ")
}

// deployBackupBucketExtension deploys the BackupBucket extension resource in Seed with the required secret.
func (a *actuator) deployBackupBucketExtension(ctx context.Context) error {
	coreSecret, err := common.GetSecretFromSecretRef(ctx, a.gardenClient, &a.backupBucket.Spec.SecretRef)
	if err != nil {
		return err
	}

	extensionSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateBackupBucketSecretName(a.backupBucket.Name),
			Namespace: v1beta1constants.GardenNamespace,
		},
	}

	if err := kutil.CreateOrUpdate(ctx, a.seedClient, extensionSecret, func() error {
		extensionSecret.Data = coreSecret.DeepCopy().Data
		return nil
	}); err != nil {
		return err
	}

	// create extension backup bucket resource in seed
	extensionBackupBucket := &extensionsv1alpha1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.backupBucket.Name,
		},
	}

	var providerConfig *runtime.RawExtension
	if a.backupBucket.Spec.ProviderConfig != nil {
		providerConfig = &a.backupBucket.Spec.ProviderConfig.RawExtension
	}

	return kutil.CreateOrUpdate(ctx, a.seedClient, extensionBackupBucket, func() error {
		metav1.SetMetaDataAnnotation(&extensionBackupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)

		extensionBackupBucket.Spec = extensionsv1alpha1.BackupBucketSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           a.backupBucket.Spec.Provider.Type,
				ProviderConfig: providerConfig,
			},
			Region: a.backupBucket.Spec.Provider.Region,
			SecretRef: corev1.SecretReference{
				Name:      generateBackupBucketSecretName(a.backupBucket.Name),
				Namespace: v1beta1constants.GardenNamespace,
			},
		}
		return nil
	})
}

// waitUntilBackupBucketExtensionReconciled waits until BackupBucket Extension resource reconciled from seed.
// It also copies the generatedSecret from seed to garden.
func (a *actuator) waitUntilBackupBucketExtensionReconciled(ctx context.Context) error {
	var backupBucket *extensionsv1alpha1.BackupBucket

	if err := retry.UntilTimeout(ctx, defaultInterval, defaultTimeout, func(ctx context.Context) (bool, error) {
		bb := &extensionsv1alpha1.BackupBucket{}
		if err := a.seedClient.Get(ctx, kutil.Key(a.backupBucket.Name), bb); err != nil {
			return retry.SevereError(err)
		}

		if err := health.CheckExtensionObject(bb); err != nil {
			a.logger.WithError(err).Error("Backup bucket did not get ready yet")
			return retry.MinorError(err)
		}

		backupBucket = bb
		return retry.Ok()
	}); err != nil {
		return gardencorev1beta1helper.DetermineError(fmt.Sprintf("Error while waiting for backupBucket object to become ready: %v", err))
	}

	var (
		generatedSecretRef *corev1.SecretReference
		providerStatus     *gardencorev1beta1.ProviderConfig
	)

	if backupBucket.Status.GeneratedSecretRef != nil {
		generatedSecret, err := common.GetSecretFromSecretRef(ctx, a.seedClient, backupBucket.Status.GeneratedSecretRef)
		if err != nil {
			return err
		}

		coreGeneratedSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generateGeneratedBackupBucketSecretName(a.backupBucket.Name),
				Namespace: v1beta1constants.GardenNamespace,
			},
		}
		ownerRef := metav1.NewControllerRef(backupBucket, gardencorev1beta1.SchemeGroupVersion.WithKind("BackupBucket"))

		if err := kutil.CreateOrUpdate(ctx, a.gardenClient, coreGeneratedSecret, func() error {
			coreGeneratedSecret.OwnerReferences = []metav1.OwnerReference{*ownerRef}
			coreGeneratedSecret.Data = generatedSecret.DeepCopy().Data

			finalizers := sets.NewString(coreGeneratedSecret.GetFinalizers()...)
			finalizers.Insert(finalizerName)
			coreGeneratedSecret.SetFinalizers(finalizers.UnsortedList())

			return nil
		}); err != nil {
			return err
		}

		generatedSecretRef = &corev1.SecretReference{
			Name:      coreGeneratedSecret.Name,
			Namespace: coreGeneratedSecret.Namespace,
		}
	}

	if backupBucket.Status.ProviderStatus != nil {
		providerStatus = &gardencorev1beta1.ProviderConfig{
			RawExtension: *backupBucket.Status.ProviderStatus,
		}
	}

	if generatedSecretRef != nil || providerStatus != nil {
		return kutil.CreateOrUpdate(ctx, a.gardenClient, a.backupBucket, func() error {
			a.backupBucket.Status.GeneratedSecretRef = generatedSecretRef
			a.backupBucket.Status.ProviderStatus = providerStatus
			return nil
		})
	}

	return nil
}

// deleteBackupBucketExtension deletes BackupBucket extension resource in seed .
func (a *actuator) deleteBackupBucketExtension(ctx context.Context) error {
	if err := a.deleteGeneratedBackupBucketSecretInGarden(ctx); err != nil {
		return err
	}

	if err := a.deleteBackupBucketExtensionSecret(ctx); err != nil {
		return err
	}

	bb := &extensionsv1alpha1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.backupBucket.Name,
		},
	}
	return client.IgnoreNotFound(a.seedClient.Delete(ctx, bb))
}

// waitUntilBackupBucketExtensionDeleted waits until backup bucket extension resource is deleted in seed cluster.
func (a *actuator) waitUntilBackupBucketExtensionDeleted(ctx context.Context) error {
	var lastError *gardencorev1beta1.LastError

	if err := retry.UntilTimeout(ctx, defaultInterval, defaultTimeout, func(ctx context.Context) (bool, error) {
		bb := &extensionsv1alpha1.BackupBucket{}
		if err := a.seedClient.Get(ctx, kutil.Key(a.backupBucket.Name), bb); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}

		if lastErr := bb.Status.LastError; lastErr != nil {
			a.logger.Errorf("BackupBucket did not get deleted yet, lastError is: %s", lastErr.Description)
			lastError = lastErr
		}

		a.logger.Infof("Waiting for backupBucket to be deleted...")
		return retry.MinorError(gardencorev1beta1helper.WrapWithLastError(fmt.Errorf("BackupBucket is still present"), lastError))
	}); err != nil {
		message := fmt.Sprintf("Error while waiting for backupBucket object to be deleted")
		if lastError != nil {
			return gardencorev1beta1helper.DetermineError(fmt.Sprintf("%s: %s", message, lastError.Description))
		}
		return gardencorev1beta1helper.DetermineError(fmt.Sprintf("%s: %s", message, err.Error()))
	}

	return nil
}

// deleteBackupBucketExtensionSecret deletes secret referred by BackupBucket extension resource in seed.
func (a *actuator) deleteBackupBucketExtensionSecret(ctx context.Context) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateBackupBucketSecretName(a.backupBucket.Name),
			Namespace: v1beta1constants.GardenNamespace,
		},
	}

	return client.IgnoreNotFound(a.seedClient.Delete(ctx, secret))
}

// deleteGeneratedBackupBucketSecretInGarden deletes generated secret referred by core BackupBucket resource in garden.
func (a *actuator) deleteGeneratedBackupBucketSecretInGarden(ctx context.Context) error {
	if a.backupBucket.Status.GeneratedSecretRef == nil {
		return nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.backupBucket.Status.GeneratedSecretRef.Name,
			Namespace: a.backupBucket.Status.GeneratedSecretRef.Namespace,
		},
	}

	if err := controllerutils.RemoveFinalizer(ctx, a.gardenClient, secret, finalizerName); err != nil {
		return err
	}
	return client.IgnoreNotFound(a.gardenClient.Delete(ctx, secret))
}

func generateBackupBucketSecretName(backupBucketName string) string {
	return fmt.Sprintf("bucket-%s", backupBucketName)
}

func generateGeneratedBackupBucketSecretName(backupBucketName string) string {
	return fmt.Sprintf("generated-bucket-%s", backupBucketName)
}
