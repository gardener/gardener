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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/extensions"
	extensionsbackupentry "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/backupentry"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// Migrate migrates the BackupEntry.
	Migrate(context.Context) error
}

type actuator struct {
	logger       *logrus.Entry
	gardenClient kubernetes.Interface
	seedClient   kubernetes.Interface
	backupBucket *gardencorev1beta1.BackupBucket
	backupEntry  *gardencorev1beta1.BackupEntry
	component    extensionsbackupentry.Interface
}

func newActuator(gardenClient, seedClient kubernetes.Interface, be *gardencorev1beta1.BackupEntry, logger logrus.FieldLogger) Actuator {
	extensionSecret := emptyExtensionSecret(be)

	return &actuator{
		logger:       logger.WithField("backupentry", be.Name),
		gardenClient: gardenClient,
		seedClient:   seedClient,
		backupBucket: &gardencorev1beta1.BackupBucket{},
		backupEntry:  be,
		component: extensionsbackupentry.New(
			logger,
			seedClient.DirectClient(),
			&extensionsbackupentry.Values{
				Name:       be.Name,
				BucketName: be.Spec.BucketName,
				SecretRef: corev1.SecretReference{
					Name:      extensionSecret.Name,
					Namespace: extensionSecret.Namespace,
				},
			},
			extensionsbackupentry.DefaultInterval,
			extensionsbackupentry.DefaultSevereThreshold,
			extensionsbackupentry.DefaultTimeout,
		),
	}
}

func (a *actuator) Reconcile(ctx context.Context) error {
	var (
		g = flow.NewGraph("Backup Entry Reconciliation")

		waitUntilBackupBucketReconciled = g.Add(flow.Task{
			Name: "Waiting until the backup bucket is reconciled",
			Fn:   a.waitUntilBackupBucketReconciled,
		})
		deployBackupEntryExtensionSecret = g.Add(flow.Task{
			Name:         "Deploying backup entry secret to seed",
			Fn:           flow.TaskFn(a.deployBackupEntryExtensionSecret).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilBackupBucketReconciled),
		})
		deployBackupEntryExtension = g.Add(flow.Task{
			Name:         "Deploying backup entry extension resource",
			Fn:           flow.TaskFn(a.deployBackupEntryExtension).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(deployBackupEntryExtensionSecret),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until backup entry is reconciled",
			Fn:           a.component.Wait,
			Dependencies: flow.NewTaskIDs(deployBackupEntryExtension),
		})

		f = g.Compile()
	)

	return f.Run(flow.Opts{
		Logger:           a.logger,
		ProgressReporter: flow.NewImmediateProgressReporter(a.reportBackupEntryProgress),
		Context:          ctx,
	})
}

func (a *actuator) Delete(ctx context.Context) error {
	var (
		g = flow.NewGraph("Backup Entry deletion")

		waitUntilBackupBucketReconciled = g.Add(flow.Task{
			Name: "Waiting until the backup bucket is reconciled",
			Fn:   a.waitUntilBackupBucketReconciled,
		})
		deployBackupEntryExtensionSecret = g.Add(flow.Task{
			Name:         "Deploying backup entry secret to seed",
			Fn:           flow.TaskFn(a.deployBackupEntryExtensionSecret).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilBackupBucketReconciled),
		})
		deleteBackupEntry = g.Add(flow.Task{
			Name:         "Destroying backup entry extension",
			Fn:           a.component.Destroy,
			Dependencies: flow.NewTaskIDs(deployBackupEntryExtensionSecret),
		})
		waitUntilBackupEntryExtensionDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension backup entry is deleted",
			Fn:           a.component.WaitCleanup,
			Dependencies: flow.NewTaskIDs(deleteBackupEntry),
		})
		_ = g.Add(flow.Task{
			Name:         "Deleting backup entry secret in seed",
			Fn:           flow.TaskFn(a.deleteBackupEntryExtensionSecret).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilBackupEntryExtensionDeleted),
		})

		f = g.Compile()
	)

	return f.Run(flow.Opts{
		Logger:           a.logger,
		ProgressReporter: flow.NewImmediateProgressReporter(a.reportBackupEntryProgress),
		Context:          ctx,
	})
}

func (a *actuator) Migrate(ctx context.Context) error {
	var (
		g = flow.NewGraph("Backup Entry migration")

		migrateBackupEntry = g.Add(flow.Task{
			Name: "Migrating backup entry extension",
			Fn:   a.component.Migrate,
		})
		waitUntilBackupEntryMigrated = g.Add(flow.Task{
			Name:         "Waiting until extension backup entry is migrated",
			Fn:           a.component.WaitMigrate,
			Dependencies: flow.NewTaskIDs(migrateBackupEntry),
		})
		deleteBackupEntry = g.Add(flow.Task{
			Name:         "Destroying backup entry extension",
			Fn:           a.component.Destroy,
			Dependencies: flow.NewTaskIDs(waitUntilBackupEntryMigrated),
		})
		waitUntilBackupEntryExtensionDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension backup entry is deleted",
			Fn:           a.component.WaitCleanup,
			Dependencies: flow.NewTaskIDs(deleteBackupEntry),
		})
		_ = g.Add(flow.Task{
			Name:         "Deleting backup entry secret in seed",
			Fn:           flow.TaskFn(a.deleteBackupEntryExtensionSecret).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilBackupEntryExtensionDeleted),
		})

		f = g.Compile()
	)

	return f.Run(flow.Opts{
		Logger:           a.logger,
		ProgressReporter: flow.NewImmediateProgressReporter(a.reportBackupEntryProgress),
		Context:          ctx,
	})
}

// reportBackupEntryProgress will update the phase and error in the BackupEntry manifest `status` section
// by the current progress of the Flow execution.
func (a *actuator) reportBackupEntryProgress(ctx context.Context, stats *flow.Stats) {
	if err := kutil.TryUpdateStatus(ctx, kretry.DefaultBackoff, a.gardenClient.DirectClient(), a.backupEntry, func() error {
		if a.backupEntry.Status.LastOperation == nil {
			return fmt.Errorf("last operation of BackupEntry %s/%s is unset", a.backupEntry.Namespace, a.backupEntry.Name)
		}
		a.backupEntry.Status.LastOperation.Description = makeDescription(stats)
		a.backupEntry.Status.LastOperation.Progress = stats.ProgressPercent()
		a.backupEntry.Status.LastOperation.LastUpdateTime = metav1.Now()
		return nil
	}); err != nil {
		a.logger.Warnf("could not report backupEntry progress with description: %s, %v", makeDescription(stats), err)
	}
}

func makeDescription(stats *flow.Stats) string {
	if stats.ProgressPercent() == 100 {
		return "Execution finished"
	}
	return strings.Join(stats.Running.StringList(), ", ")
}

func (a *actuator) waitUntilBackupBucketReconciled(ctx context.Context) error {
	if err := extensions.WaitUntilObjectReadyWithHealthFunction(
		ctx,
		a.gardenClient.DirectClient(),
		a.logger,
		health.CheckBackupBucket,
		func() client.Object { return &gardencorev1beta1.BackupBucket{} },
		extensionsv1alpha1.BackupBucketResource,
		"",
		a.backupEntry.Spec.BucketName,
		defaultInterval,
		defaultSevereThreshold,
		defaultTimeout,
		func(obj client.Object) error {
			bb, ok := obj.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return fmt.Errorf("expected gardencorev1beta1.BackupBucket but got %T", obj)
			}
			bb.DeepCopyInto(a.backupBucket)
			return nil
		},
	); err != nil {
		a.logger.Errorf("associated BackupBucket %s is not ready yet with err: %v", a.backupEntry.Spec.BucketName, err)
		return err
	}

	return nil
}

func emptyExtensionSecret(backupEntry *gardencorev1beta1.BackupEntry) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("entry-%s", backupEntry.Name),
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
}

func (a *actuator) deployBackupEntryExtensionSecret(ctx context.Context) error {
	coreSecretRef := &a.backupBucket.Spec.SecretRef
	if a.backupBucket.Status.GeneratedSecretRef != nil {
		coreSecretRef = a.backupBucket.Status.GeneratedSecretRef
	}

	coreSecret, err := kutil.GetSecretByReference(ctx, a.gardenClient.Client(), coreSecretRef)
	if err != nil {
		return errors.Wrapf(err, "could not get secret referred in core backup bucket")
	}

	// create secret for extension BackupEntry in seed
	extensionSecret := emptyExtensionSecret(a.backupEntry)
	if _, err := controllerutil.CreateOrUpdate(ctx, a.seedClient.Client(), extensionSecret, func() error {
		extensionSecret.Data = coreSecret.DeepCopy().Data
		return nil
	}); err != nil {
		return errors.Wrapf(err, "could not reconcile extension secret in seed")
	}

	return nil
}

// deleteBackupEntryExtensionSecret deletes secret referred by BackupEntry extension resource in seed.
func (a *actuator) deleteBackupEntryExtensionSecret(ctx context.Context) error {
	return client.IgnoreNotFound(a.seedClient.Client().Delete(ctx, emptyExtensionSecret(a.backupEntry)))
}

// deployBackupEntryExtension deploys the BackupEntry extension resource in Seed with the required secret.
func (a *actuator) deployBackupEntryExtension(ctx context.Context) error {
	a.component.SetType(a.backupBucket.Spec.Provider.Type)
	a.component.SetProviderConfig(a.backupBucket.Spec.ProviderConfig)
	a.component.SetRegion(a.backupBucket.Spec.Provider.Region)
	a.component.SetBackupBucketProviderStatus(a.backupBucket.Status.ProviderStatus)

	if !a.isRestorePhase() {
		return a.component.Deploy(ctx)
	}

	shootName := common.GetShootNameFromOwnerReferences(a.backupEntry.OwnerReferences)
	shootState := &gardencorev1alpha1.ShootState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shootName,
			Namespace: a.backupEntry.Namespace,
		},
	}
	if err := a.gardenClient.Client().Get(ctx, kutil.Key(shootState.Namespace, shootState.Name), shootState); err != nil {
		return err
	}
	return a.component.Restore(ctx, shootState)
}

// isRestorePhase checks to see if the BackupEntry's LastOperation is Restore
func (a *actuator) isRestorePhase() bool {
	return a.backupEntry.Status.LastOperation != nil && a.backupEntry.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeRestore
}
