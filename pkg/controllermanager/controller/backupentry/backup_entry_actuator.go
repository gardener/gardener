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
	"fmt"
	"strings"
	"time"

	controllerutils "github.com/gardener/gardener/pkg/controllermanager/controller/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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

// Actuator acts upon BackupEntry resources.
type Actuator interface {
	// Reconcile reconciles the BackupEntry.
	Reconcile(context.Context) error
	// Delete deletes the BackupEntry.
	Delete(context.Context) error
}

type actuator struct {
	gardenClient client.Client
	seedClient   client.Client
	logger       logrus.FieldLogger
	backupEntry  *gardencorev1alpha1.BackupEntry
}

func newActuator(gardenClient, seedClient client.Client, be *gardencorev1alpha1.BackupEntry, logger logrus.FieldLogger) Actuator {
	return &actuator{
		logger:       logger.WithField("backupentry", be.Name),
		backupEntry:  be,
		gardenClient: gardenClient,
		seedClient:   seedClient,
	}
}

func (a *actuator) Reconcile(ctx context.Context) error {
	var (
		g = flow.NewGraph("Backup Entry Creation")

		deployBackupEntryExtension = g.Add(flow.Task{
			Name: "Deploying backup entry extension resource",
			Fn:   flow.TaskFn(a.deployBackupEntryExtension).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})

		_ = g.Add(flow.Task{
			Name:         "Waiting until backup entry is reconciled",
			Fn:           flow.TaskFn(a.waitUntilBackupEntryExtensionReconciled),
			Dependencies: flow.NewTaskIDs(deployBackupEntryExtension),
		})
		f = g.Compile()
	)
	return f.Run(flow.Opts{
		Logger:           a.logger,
		ProgressReporter: a.reportBackupEntryProgress,
		Context:          ctx,
	})
}

func (a *actuator) Delete(ctx context.Context) error {
	var (
		g                 = flow.NewGraph("Backup entry deletion")
		deleteBackupEntry = g.Add(flow.Task{
			Name: "Destroying backup entry extension",
			Fn:   flow.TaskFn(a.deleteBackupEntryExtension),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until extension backup entry is deleted",
			Fn:           flow.TaskFn(a.waitUntilBackupEntryExtensionDeleted),
			Dependencies: flow.NewTaskIDs(deleteBackupEntry),
		})
		f = g.Compile()
	)
	return f.Run(flow.Opts{
		Logger:           a.logger,
		ProgressReporter: a.reportBackupEntryProgress,
		Context:          ctx,
	})
}

// reportBackupEntryProgress will update the phase and error in the BackupEntry manifest `status` section
// by the current progress of the Flow execution.
func (a *actuator) reportBackupEntryProgress(ctx context.Context, stats *flow.Stats) {
	if err := kutil.TryUpdateStatus(ctx, kretry.DefaultRetry, a.gardenClient, a.backupEntry, func() error {
		a.backupEntry.Status.LastOperation.Description = makeDescription(stats)
		a.backupEntry.Status.LastOperation.Progress = stats.ProgressPercent()
		a.backupEntry.Status.LastOperation.LastUpdateTime = metav1.Now()
		return nil
	}); err != nil {
		a.logger.Warnf("could not report backupEntry progress with description: %s", makeDescription(stats))
	}
}

func makeDescription(stats *flow.Stats) string {
	if stats.ProgressPercent() == 100 {
		return "Execution finished"
	}
	return strings.Join(stats.Running.StringList(), ", ")
}

// deployBackupEntryExtension deploys the BackupEntry extension resource in Seed with the required secret.
func (a *actuator) deployBackupEntryExtension(ctx context.Context) error {
	bb := &gardencorev1alpha1.BackupBucket{}
	if err := a.waitUntilCoreBackupBucketReconciled(ctx, bb); err != nil {
		a.logger.Errorf("associated backupBucket %s is not ready yet with err: %v ", a.backupEntry.Spec.BucketName, err)
		return err
	}

	coreSecretRef := &bb.Spec.SecretRef
	if bb.Status.GeneratedSecretRef != nil {
		coreSecretRef = bb.Status.GeneratedSecretRef
	}

	coreSecret, err := common.GetSecretFromSecretRef(ctx, a.gardenClient, coreSecretRef)
	if err != nil {
		return err
	}

	// create secret for extension backup Entry in seed
	extensionSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateBackupEntrySecretName(a.backupEntry.Name),
			Namespace: common.GardenNamespace,
		},
	}

	if err := kutil.CreateOrUpdate(ctx, a.seedClient, extensionSecret, func() error {
		extensionSecret.Data = coreSecret.DeepCopy().Data
		finalizers := sets.NewString(extensionSecret.GetFinalizers()...)
		finalizers.Insert(finalizerName)
		extensionSecret.SetFinalizers(finalizers.UnsortedList())
		return nil
	}); err != nil {
		return err
	}

	// create extension backupEntry resource in seed
	extensionbackupEntry := &extensionsv1alpha1.BackupEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.backupEntry.Name,
		},
	}

	return kutil.CreateOrUpdate(ctx, a.seedClient, extensionbackupEntry, func() error {
		extensionbackupEntry.ObjectMeta.Annotations = map[string]string{
			common.SecretRefChecksumAnnotation: bb.Annotations[common.SecretRefChecksumAnnotation],
		}
		extensionbackupEntry.Spec = extensionsv1alpha1.BackupEntrySpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: bb.Spec.Provider.Type,
			},
			BucketName: a.backupEntry.Spec.BucketName,
			Region:     bb.Spec.Provider.Region,
			SecretRef: corev1.SecretReference{
				Name:      extensionSecret.Name,
				Namespace: extensionSecret.Namespace,
			},
		}
		return nil
	})
}

// waitUntilCoreBackupBucketReconciled waits until core.BackupBucket resource reconciled from seed.
func (a *actuator) waitUntilCoreBackupBucketReconciled(ctx context.Context, bb *gardencorev1alpha1.BackupBucket) error {
	if err := retry.UntilTimeout(ctx, defaultInterval, defaultTimeout, func(ctx context.Context) (bool, error) {
		if err := a.gardenClient.Get(ctx, kutil.Key(a.backupEntry.Spec.BucketName), bb); err != nil {
			return retry.SevereError(err)
		}

		if err := health.CheckBackupBucket(bb); err != nil {
			a.logger.WithError(err).Error("BackupBucket did not get ready yet")
			return retry.MinorError(err)
		}
		return retry.Ok()
	}); err != nil {
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("Error while waiting for backupEntry object to become ready: %v", err))
	}
	return nil
}

// waitUntilBackupEntryExtensionReconciled waits until BackupEntry Extention resource reconciled from seed.
func (a *actuator) waitUntilBackupEntryExtensionReconciled(ctx context.Context) error {
	if err := retry.UntilTimeout(ctx, defaultInterval, defaultTimeout, func(ctx context.Context) (bool, error) {
		be := &extensionsv1alpha1.BackupEntry{}
		if err := a.seedClient.Get(ctx, kutil.Key(a.backupEntry.Namespace, a.backupEntry.Name), be); err != nil {
			return retry.SevereError(err)
		}

		if err := health.CheckExtensionObject(be); err != nil {
			a.logger.WithError(err).Error("BackupEntry did not get ready yet")
			return retry.MinorError(err)
		}
		return retry.Ok()
	}); err != nil {
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("Error while waiting for backupEntry object to become ready: %v", err))
	}
	return nil
}

// deleteBackupEntryExtension deletes BackupEntry extension resource in seed.
func (a *actuator) deleteBackupEntryExtension(ctx context.Context) error {
	be := &extensionsv1alpha1.BackupEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.backupEntry.Name,
			Namespace: a.backupEntry.Namespace,
		},
	}

	return client.IgnoreNotFound(a.seedClient.Delete(ctx, be))
}

// waitUntilBackupEntryExtensionDeleted waits until backup entry extension resource is deleted in seed cluster.
func (a *actuator) waitUntilBackupEntryExtensionDeleted(ctx context.Context) error {
	var lastError *gardencorev1alpha1.LastError

	if err := retry.UntilTimeout(ctx, defaultInterval, defaultTimeout, func(ctx context.Context) (bool, error) {
		be := &extensionsv1alpha1.BackupEntry{}
		if err := a.seedClient.Get(ctx, kutil.Key(a.backupEntry.Namespace, a.backupEntry.Name), be); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}

		if lastErr := be.Status.LastError; lastErr != nil {
			a.logger.Errorf("BackupEntry did not get deleted yet, lastError is: %s", lastErr.Description)
			lastError = lastErr
		}

		a.logger.Infof("Waiting for backupEntry to be deleted...")
		return retry.MinorError(common.WrapWithLastError(fmt.Errorf("backupEntry is still present"), lastError))
	}); err != nil {
		message := fmt.Sprintf("Error while waiting for backupEntry object to be deleted")
		if lastError != nil {
			return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, lastError.Description))
		}
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, err.Error()))
	}

	return a.deleteBackupEntryExtensionSecret(ctx)
}

// deleteBackupEntryExtensionSecret deletes secret referred by BackupEntry extension resource in seed.
func (a *actuator) deleteBackupEntryExtensionSecret(ctx context.Context) error {
	extensionSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateBackupEntrySecretName(a.backupEntry.Name),
			Namespace: common.GardenNamespace,
		},
	}
	if err := client.IgnoreNotFound(a.seedClient.Delete(ctx, extensionSecret)); err != nil {
		return err
	}

	return controllerutils.RemoveFinalizer(ctx, a.seedClient, extensionSecret, finalizerName)
}

func generateBackupEntrySecretName(backupEntryName string) string {
	return fmt.Sprintf("entry-%s", backupEntryName)
}
