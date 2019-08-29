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

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	controllerutils "github.com/gardener/gardener/pkg/controllermanager/controller/utils"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	gardenClient           client.Client
	seedClient             client.Client
	logger                 logrus.FieldLogger
	backupBucket           *gardencorev1alpha1.BackupBucket
	coreGeneratedSecretRef *corev1.SecretReference
}

func newActuator(gardenClient, seedClient client.Client, bb *gardencorev1alpha1.BackupBucket, logger logrus.FieldLogger) Actuator {
	return &actuator{
		logger:       logger.WithField("backupbucket", bb.Name),
		backupBucket: bb,
		gardenClient: gardenClient,
		seedClient:   seedClient,
	}
}

func (a *actuator) Reconcile(ctx context.Context) error {
	var (
		g = flow.NewGraph("Backup Bucket Creation")

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
		a.backupBucket.Status.GeneratedSecretRef = a.coreGeneratedSecretRef
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
	extensionSecretRef := &corev1.SecretReference{
		Name:      generateBackupBucketSecretName(a.backupBucket.Name),
		Namespace: common.GardenNamespace,
	}
	if err := copySecret(ctx, a.gardenClient, a.seedClient, &a.backupBucket.Spec.SecretRef, extensionSecretRef, finalizerName); err != nil {
		return err
	}

	var generatedExtensionSecretRef *corev1.SecretReference
	if a.backupBucket.Status.GeneratedSecretRef != nil {
		generatedExtensionSecretRef = &corev1.SecretReference{
			Name:      generateGeneratedBackupBucketSecretName(a.backupBucket.Name),
			Namespace: common.GardenNamespace,
		}
		if err := copySecret(ctx, a.gardenClient, a.seedClient, a.backupBucket.Status.GeneratedSecretRef, generatedExtensionSecretRef, ""); err != nil {
			return err
		}
	}

	// create extension backup bucket resource in seed
	extensionbackupBucket := &extensionsv1alpha1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.backupBucket.Name,
		},
	}

	return kutil.CreateOrUpdate(ctx, a.seedClient, extensionbackupBucket, func() error {
		extensionbackupBucket.ObjectMeta.Annotations = map[string]string{
			common.SecretRefChecksumAnnotation: a.backupBucket.Annotations[common.SecretRefChecksumAnnotation],
		}
		extensionbackupBucket.Spec = extensionsv1alpha1.BackupBucketSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: a.backupBucket.Spec.Provider.Type,
			},
			Region:    a.backupBucket.Spec.Provider.Region,
			SecretRef: *extensionSecretRef,
		}
		extensionbackupBucket.Status.GeneratedSecretRef = generatedExtensionSecretRef
		return nil
	})
}

// waitUntilBackupBucketExtensionReconciled waits until BackupBucket Extention resource reconciled from seed.
// It also copies the generatedSecret from seed to garden.
func (a *actuator) waitUntilBackupBucketExtensionReconciled(ctx context.Context) error {
	bb := &extensionsv1alpha1.BackupBucket{}
	if err := retry.UntilTimeout(ctx, defaultInterval, defaultTimeout, func(ctx context.Context) (bool, error) {
		if err := a.seedClient.Get(ctx, kutil.Key(a.backupBucket.Name), bb); err != nil {
			return retry.SevereError(err)
		}
		if err := health.CheckExtensionObject(bb); err != nil {
			a.logger.WithError(err).Error("Backup bucket did not get ready yet")
			return retry.MinorError(err)
		}
		return retry.Ok()
	}); err != nil {
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("Error while waiting for backupBucket object to become ready: %v", err))
	}

	if bb.Status.GeneratedSecretRef != nil {
		coreGeneratedSecretRef := &corev1.SecretReference{
			Name:      generateGeneratedBackupBucketSecretName(a.backupBucket.Name),
			Namespace: common.GardenNamespace,
		}
		if err := copySecret(ctx, a.seedClient, a.gardenClient, bb.Status.GeneratedSecretRef, coreGeneratedSecretRef, finalizerName); err != nil {
			return err
		}

		coreGeneratedSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      coreGeneratedSecretRef.Name,
				Namespace: coreGeneratedSecretRef.Namespace,
			},
		}
		ownerRef := metav1.NewControllerRef(bb, gardencorev1alpha1.SchemeGroupVersion.WithKind("BackupBucket"))

		if err := kutil.CreateOrUpdate(ctx, a.gardenClient, coreGeneratedSecret, func() error {
			coreGeneratedSecret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
			return nil
		}); err != nil {
			return err
		}
		a.coreGeneratedSecretRef = coreGeneratedSecretRef
	}
	return nil
}

// deleteBackupBucketExtension deletes BackupBucket extension resource in seed .
func (a *actuator) deleteBackupBucketExtension(ctx context.Context) error {
	bb := &extensionsv1alpha1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.backupBucket.Name,
		},
	}
	return client.IgnoreNotFound(a.seedClient.Delete(ctx, bb))
}

// waitUntilBackupBucketExtensionDeleted waits until backup bucket extension resource is deleted in seed cluster.
func (a *actuator) waitUntilBackupBucketExtensionDeleted(ctx context.Context) error {
	var lastError *gardencorev1alpha1.LastError

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
		return retry.MinorError(common.WrapWithLastError(fmt.Errorf("worker is still present"), lastError))
	}); err != nil {
		message := fmt.Sprintf("Error while waiting for backupBucket object to be deleted")
		if lastError != nil {
			return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, lastError.Description))
		}
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, err.Error()))
	}

	if err := a.deleteGenerateBackupBucketSecret(ctx); err != nil {
		return err
	}

	return a.deleteBackupBucketExtensionSecret(ctx)
}

// deleteBackupBucketExtensionSecret deletes secret referred by BackupBucket extension resource in seed.
func (a *actuator) deleteBackupBucketExtensionSecret(ctx context.Context) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateBackupBucketSecretName(a.backupBucket.Name),
			Namespace: common.GardenNamespace,
		},
	}
	if err := controllerutils.RemoveFinalizer(ctx, a.seedClient, secret, finalizerName); err != nil {
		return err
	}

	return client.IgnoreNotFound(a.seedClient.Delete(ctx, secret))
}

// deleteGenerateBackupBucketSecret deletes generated secret referred by core BackupBucket resource in garden.
func (a *actuator) deleteGenerateBackupBucketSecret(ctx context.Context) error {
	if a.backupBucket.Status.GeneratedSecretRef != nil {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      a.backupBucket.Status.GeneratedSecretRef.Name,
				Namespace: a.backupBucket.Status.GeneratedSecretRef.Namespace,
			},
		}
		if err := controllerutils.RemoveFinalizer(ctx, a.seedClient, secret, finalizerName); err != nil {
			return err
		}

		return client.IgnoreNotFound(a.gardenClient.Delete(ctx, secret))
	}
	return nil
}

func generateBackupBucketSecretName(backupBucketName string) string {
	return fmt.Sprintf("bucket-%s", backupBucketName)
}

func generateGeneratedBackupBucketSecretName(backupBucketName string) string {
	return fmt.Sprintf("generated-bucket-%s", backupBucketName)
}

func copySecret(ctx context.Context, srcClient, dstClient client.Client, srcSecretRef, dstSecretRef *corev1.SecretReference, withFinalizer string) error {
	srcSecret, err := common.GetSecretFromSecretRef(ctx, srcClient, srcSecretRef)
	if err != nil {
		return err
	}

	dstSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dstSecretRef.Name,
			Namespace: dstSecretRef.Namespace,
		},
	}

	return kutil.CreateOrUpdate(ctx, dstClient, dstSecret, func() error {
		dstSecret.Data = srcSecret.DeepCopy().Data
		if len(finalizerName) != 0 {
			finalizers := sets.NewString(dstSecret.GetFinalizers()...)
			finalizers.Insert(withFinalizer)
			dstSecret.SetFinalizers(finalizers.UnsortedList())
		}
		return nil
	})
}
