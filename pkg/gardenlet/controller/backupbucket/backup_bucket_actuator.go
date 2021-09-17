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
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	defaultTimeout         = 30 * time.Second
	defaultInterval        = 5 * time.Second
	defaultSevereThreshold = 15 * time.Second
)

// Actuator acts upon BackupBucket resources.
type Actuator interface {
	// Reconcile reconciles the BackupBucket.
	Reconcile(context.Context) error
	// Delete deletes the BackupBucket.
	Delete(context.Context) error
}

type actuator struct {
	gardenClient kubernetes.Interface
	seedClient   kubernetes.Interface
	logger       *logrus.Entry

	backupBucket          *gardencorev1beta1.BackupBucket
	extensionBackupBucket *extensionsv1alpha1.BackupBucket
}

func newActuator(gardenClient, seedClient kubernetes.Interface, bb *gardencorev1beta1.BackupBucket, logger logrus.FieldLogger) Actuator {
	return &actuator{
		logger:       logger.WithField("backupbucket", bb.Name),
		gardenClient: gardenClient,
		seedClient:   seedClient,

		backupBucket: bb,
		extensionBackupBucket: &extensionsv1alpha1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: bb.Name,
			},
		},
	}
}

func (a *actuator) Reconcile(ctx context.Context) error {
	var (
		g = flow.NewGraph("Backup Bucket Reconciliation")

		deployBackupBucketSecret = g.Add(flow.Task{
			Name: "Deploying backup bucket secret to seed",
			Fn:   flow.TaskFn(a.deployBackupBucketExtensionSecret).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		deployBackupBucketExtension = g.Add(flow.Task{
			Name:         "Deploying backup bucket extension resource",
			Fn:           flow.TaskFn(a.deployBackupBucketExtension).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployBackupBucketSecret),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until backup bucket is reconciled",
			Fn:           a.waitUntilBackupBucketExtensionReconciled,
			Dependencies: flow.NewTaskIDs(deployBackupBucketExtension),
		})

		f = g.Compile()
	)

	return f.Run(ctx, flow.Opts{
		Logger:           a.logger,
		ProgressReporter: flow.NewImmediateProgressReporter(a.reportBackupBucketProgress),
	})
}

func (a *actuator) Delete(ctx context.Context) error {
	var (
		g = flow.NewGraph("Backup bucket deletion")

		_ = g.Add(flow.Task{
			Name: "Destroying generated backup bucket secret in garden cluster",
			Fn:   a.deleteGeneratedBackupBucketSecretInGarden,
		})
		deployBackupBucketSecret = g.Add(flow.Task{
			Name: "Deploying backup bucket secret to seed",
			Fn:   flow.TaskFn(a.deployBackupBucketExtensionSecret).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		deleteBackupBucket = g.Add(flow.Task{
			Name:         "Destroying backup bucket",
			Fn:           a.deleteBackupBucketExtension,
			Dependencies: flow.NewTaskIDs(deployBackupBucketSecret),
		})
		waitUntilBackupBucketExtensionDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension backup bucket is deleted",
			Fn:           a.waitUntilBackupBucketExtensionDeleted,
			Dependencies: flow.NewTaskIDs(deleteBackupBucket),
		})
		_ = g.Add(flow.Task{
			Name:         "Deleting backup bucket secret in seed",
			Fn:           flow.TaskFn(a.deleteBackupBucketExtensionSecret).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilBackupBucketExtensionDeleted),
		})

		f = g.Compile()
	)

	return f.Run(ctx, flow.Opts{
		Logger:           a.logger,
		ProgressReporter: flow.NewImmediateProgressReporter(a.reportBackupBucketProgress),
	})
}

// reportBackupBucketProgress will update the phase and error in the BackupBucket manifest `status` section
// by the current progress of the Flow execution.
func (a *actuator) reportBackupBucketProgress(ctx context.Context, stats *flow.Stats) {
	patch := client.MergeFrom(a.backupBucket.DeepCopy())

	if a.backupBucket.Status.LastOperation == nil {
		a.backupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{}
	}
	a.backupBucket.Status.LastOperation.Description = makeDescription(stats)
	a.backupBucket.Status.LastOperation.Progress = stats.ProgressPercent()
	a.backupBucket.Status.LastOperation.LastUpdateTime = metav1.Now()

	if err := a.gardenClient.Client().Status().Patch(ctx, a.backupBucket, patch); err != nil {
		a.logger.Warnf("could not report backupbucket progress with description: %s: %v", makeDescription(stats), err)
	}
}

func makeDescription(stats *flow.Stats) string {
	if stats.ProgressPercent() == 100 {
		return "Execution finished"
	}
	return strings.Join(stats.Running.StringList(), ", ")
}

func (a *actuator) emptyExtensionSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateBackupBucketSecretName(a.backupBucket.Name),
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
}

func (a *actuator) deployBackupBucketExtensionSecret(ctx context.Context) error {
	coreSecret, err := kutil.GetSecretByReference(ctx, a.gardenClient.Client(), &a.backupBucket.Spec.SecretRef)
	if err != nil {
		return err
	}

	extensionSecret := a.emptyExtensionSecret()
	_, err = controllerutils.GetAndCreateOrMergePatch(ctx, a.seedClient.Client(), extensionSecret, func() error {
		extensionSecret.Data = coreSecret.DeepCopy().Data
		return nil
	})
	return err
}

// deployBackupBucketExtension deploys the BackupBucket extension resource in Seed with the required secret.
func (a *actuator) deployBackupBucketExtension(ctx context.Context) error {
	extensionSecret := a.emptyExtensionSecret()

	// reconcile extension backup bucket resource in seed
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, a.seedClient.Client(), a.extensionBackupBucket, func() error {
		metav1.SetMetaDataAnnotation(&a.extensionBackupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&a.extensionBackupBucket.ObjectMeta, v1beta1constants.GardenerTimestamp, time.Now().UTC().String())

		a.extensionBackupBucket.Spec = extensionsv1alpha1.BackupBucketSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           a.backupBucket.Spec.Provider.Type,
				ProviderConfig: a.backupBucket.Spec.ProviderConfig,
			},
			Region: a.backupBucket.Spec.Provider.Region,
			SecretRef: corev1.SecretReference{
				Name:      extensionSecret.Name,
				Namespace: extensionSecret.Namespace,
			},
		}
		return nil
	})
	return err
}

// waitUntilBackupBucketExtensionReconciled waits until BackupBucket Extension resource reconciled from seed.
// It also copies the generatedSecret from seed to garden.
func (a *actuator) waitUntilBackupBucketExtensionReconciled(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectReady(
		ctx,
		a.seedClient.Client(),
		a.logger,
		a.extensionBackupBucket,
		extensionsv1alpha1.BackupBucketResource,
		defaultInterval,
		defaultSevereThreshold,
		defaultTimeout,
		func() error {
			var generatedSecretRef *corev1.SecretReference

			if a.extensionBackupBucket.Status.GeneratedSecretRef != nil {
				generatedSecret, err := kutil.GetSecretByReference(ctx, a.seedClient.Client(), a.extensionBackupBucket.Status.GeneratedSecretRef)
				if err != nil {
					return err
				}

				coreGeneratedSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      generateGeneratedBackupBucketSecretName(a.backupBucket.Name),
						Namespace: v1beta1constants.GardenNamespace,
					},
				}
				ownerRef := metav1.NewControllerRef(a.extensionBackupBucket, gardencorev1beta1.SchemeGroupVersion.WithKind("BackupBucket"))

				if _, err := controllerutils.CreateOrGetAndStrategicMergePatch(ctx, a.gardenClient.Client(), coreGeneratedSecret, func() error {
					coreGeneratedSecret.OwnerReferences = []metav1.OwnerReference{*ownerRef}
					controllerutil.AddFinalizer(coreGeneratedSecret, finalizerName)
					coreGeneratedSecret.Data = generatedSecret.DeepCopy().Data
					return nil
				}); err != nil {
					return err
				}

				generatedSecretRef = &corev1.SecretReference{
					Name:      coreGeneratedSecret.Name,
					Namespace: coreGeneratedSecret.Namespace,
				}
			}

			if generatedSecretRef != nil || a.extensionBackupBucket.Status.ProviderStatus != nil {
				patch := client.MergeFrom(a.backupBucket.DeepCopy())
				a.backupBucket.Status.GeneratedSecretRef = generatedSecretRef
				a.backupBucket.Status.ProviderStatus = a.extensionBackupBucket.Status.ProviderStatus
				return a.gardenClient.Client().Status().Patch(ctx, a.backupBucket, patch)
			}

			return nil
		})
}

// deleteBackupBucketExtension deletes BackupBucket extension resource in seed.
func (a *actuator) deleteBackupBucketExtension(ctx context.Context) error {
	return extensions.DeleteExtensionObject(
		ctx,
		a.seedClient.Client(),
		a.extensionBackupBucket,
	)
}

// waitUntilBackupBucketExtensionDeleted waits until backup bucket extension resource is deleted in seed cluster.
func (a *actuator) waitUntilBackupBucketExtensionDeleted(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		a.seedClient.Client(),
		a.logger,
		a.extensionBackupBucket,
		extensionsv1alpha1.BackupBucketResource,
		defaultInterval,
		defaultTimeout,
	)
}

// deleteBackupBucketExtensionSecret deletes secret referred by BackupBucket extension resource in seed.
func (a *actuator) deleteBackupBucketExtensionSecret(ctx context.Context) error {
	return client.IgnoreNotFound(a.seedClient.Client().Delete(ctx, a.emptyExtensionSecret()))
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

	err := a.gardenClient.Client().Get(ctx, client.ObjectKeyFromObject(secret), secret)
	if err == nil {
		if err2 := controllerutils.PatchRemoveFinalizers(ctx, a.gardenClient.Client(), secret, finalizerName); err2 != nil {
			return fmt.Errorf("failed to remove finalizer from BackupBucket generated secret '%s/%s': %w", secret.Namespace, secret.Name, err2)
		}
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get BackupBucket generated secret '%s/%s': %w", secret.Namespace, secret.Name, err)
	}

	return client.IgnoreNotFound(a.gardenClient.Client().Delete(ctx, secret))
}

func generateBackupBucketSecretName(backupBucketName string) string {
	return fmt.Sprintf("bucket-%s", backupBucketName)
}

func generateGeneratedBackupBucketSecretName(backupBucketName string) string {
	return v1beta1constants.SecretPrefixGeneratedBackupBucket + backupBucketName
}
