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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kretry "k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	defaultTimeout         = 30 * time.Second
	defaultSevereThreshold = 15 * time.Second
	defaultInterval        = 5 * time.Second
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
	logger       *logrus.Entry
	backupEntry  *gardencorev1beta1.BackupEntry
}

func newActuator(gardenClient, seedClient client.Client, be *gardencorev1beta1.BackupEntry, logger logrus.FieldLogger) Actuator {
	return &actuator{
		logger:       logger.WithField("backupentry", be.Name),
		backupEntry:  be,
		gardenClient: gardenClient,
		seedClient:   seedClient,
	}
}

func (a *actuator) Reconcile(ctx context.Context) error {
	var (
		g = flow.NewGraph("Backup Entry Reconciliation")

		deployBackupEntryExtension = g.Add(flow.Task{
			Name: "Deploying backup entry extension resource",
			Fn:   flow.TaskFn(a.deployBackupEntryExtension).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until backup entry is reconciled",
			Fn:           a.waitUntilBackupEntryExtensionReconciled,
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
		g = flow.NewGraph("Backup Entry deletion")

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

// waitUntilCoreBackupBucketReconciled waits until core.BackupBucket resource reconciled from seed.
func (a *actuator) waitUntilCoreBackupBucketReconciled(ctx context.Context, backupBucket *gardencorev1beta1.BackupBucket) error {
	return common.WaitUntilObjectReadyWithHealthFunction(
		ctx,
		a.gardenClient,
		a.logger,
		health.CheckBackupBucket,
		func() runtime.Object { return &gardencorev1beta1.BackupBucket{} },
		"BackupBucket",
		"",
		a.backupEntry.Spec.BucketName,
		defaultInterval,
		defaultSevereThreshold,
		defaultTimeout,
		func(obj runtime.Object) error {
			bb, ok := obj.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return fmt.Errorf("expected gardencorev1beta1.BackupBucket but got %T", obj)
			}
			bb.DeepCopyInto(backupBucket)
			return nil
		},
	)
}

// deployBackupEntryExtension deploys the BackupEntry extension resource in Seed with the required secret.
func (a *actuator) deployBackupEntryExtension(ctx context.Context) error {
	bb := &gardencorev1beta1.BackupBucket{}
	if err := a.waitUntilCoreBackupBucketReconciled(ctx, bb); err != nil {
		a.logger.Errorf("associated BackupBucket %s is not ready yet with err: %v", a.backupEntry.Spec.BucketName, err)
		return err
	}

	coreSecretRef := &bb.Spec.SecretRef
	if bb.Status.GeneratedSecretRef != nil {
		coreSecretRef = bb.Status.GeneratedSecretRef
	}

	coreSecret, err := common.GetSecretFromSecretRef(ctx, a.gardenClient, coreSecretRef)
	if err != nil {
		return errors.Wrapf(err, "could not get secret referred in core backup bucket")
	}

	// create secret for extension BackupEntry in seed
	extensionSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateBackupEntrySecretName(a.backupEntry.Name),
			Namespace: v1beta1constants.GardenNamespace,
		},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, a.seedClient, extensionSecret, func() error {
		extensionSecret.Data = coreSecret.DeepCopy().Data
		return nil
	}); err != nil {
		return errors.Wrapf(err, "could not reconcile extension secret in seed")
	}

	// create extension BackupEntry resource in seed
	extensionBackupEntry := &extensionsv1alpha1.BackupEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.backupEntry.Name,
		},
	}

	var (
		backupBucketProviderConfig *runtime.RawExtension
		backupBucketProviderStatus *runtime.RawExtension
	)

	if bb.Spec.ProviderConfig != nil {
		backupBucketProviderConfig = &bb.Spec.ProviderConfig.RawExtension
	}
	if bb.Status.ProviderStatus != nil {
		backupBucketProviderStatus = &bb.Status.ProviderStatus.RawExtension
	}

	_, err = controllerutil.CreateOrUpdate(ctx, a.seedClient, extensionBackupEntry, func() error {
		metav1.SetMetaDataAnnotation(&extensionBackupEntry.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&extensionBackupEntry.ObjectMeta, v1beta1constants.GardenerTimestamp, time.Now().UTC().String())

		extensionBackupEntry.Spec = extensionsv1alpha1.BackupEntrySpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           bb.Spec.Provider.Type,
				ProviderConfig: backupBucketProviderConfig,
			},
			BackupBucketProviderStatus: backupBucketProviderStatus,
			BucketName:                 a.backupEntry.Spec.BucketName,
			Region:                     bb.Spec.Provider.Region,
			SecretRef: corev1.SecretReference{
				Name:      extensionSecret.Name,
				Namespace: extensionSecret.Namespace,
			},
		}
		return nil
	})
	return err
}

// waitUntilBackupEntryExtensionReconciled waits until BackupEntry Extension resource reconciled from seed.
func (a *actuator) waitUntilBackupEntryExtensionReconciled(ctx context.Context) error {
	return common.WaitUntilExtensionCRReady(
		ctx,
		a.seedClient,
		a.logger,
		func() runtime.Object { return &extensionsv1alpha1.BackupEntry{} },
		"BackupEntry",
		a.backupEntry.Namespace,
		a.backupEntry.Name,
		defaultInterval,
		defaultSevereThreshold,
		defaultTimeout,
		nil,
	)
}

// deleteBackupEntryExtension deletes BackupEntry extension resource in seed.
func (a *actuator) deleteBackupEntryExtension(ctx context.Context) error {
	if err := a.deleteBackupEntryExtensionSecret(ctx); err != nil {
		return err
	}

	return common.DeleteExtensionCR(
		ctx,
		a.seedClient,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.BackupEntry{} },
		a.backupEntry.Namespace,
		a.backupEntry.Name,
	)
}

// waitUntilBackupEntryExtensionDeleted waits until backup entry extension resource is deleted in seed cluster.
func (a *actuator) waitUntilBackupEntryExtensionDeleted(ctx context.Context) error {
	return common.WaitUntilExtensionCRDeleted(
		ctx,
		a.seedClient,
		a.logger,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.BackupEntry{} },
		"BackupEntry",
		a.backupEntry.Namespace,
		a.backupEntry.Name,
		defaultInterval,
		defaultTimeout,
	)
}

// deleteBackupEntryExtensionSecret deletes secret referred by BackupEntry extension resource in seed.
func (a *actuator) deleteBackupEntryExtensionSecret(ctx context.Context) error {
	extensionSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateBackupEntrySecretName(a.backupEntry.Name),
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
	return client.IgnoreNotFound(a.seedClient.Delete(ctx, extensionSecret))
}

func generateBackupEntrySecretName(backupEntryName string) string {
	return fmt.Sprintf("entry-%s", backupEntryName)
}
