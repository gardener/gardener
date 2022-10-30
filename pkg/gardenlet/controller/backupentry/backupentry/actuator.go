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

	"github.com/go-logr/logr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	extensionsbackupentry "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/backupentry"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	DefaultTimeout                   = 30 * time.Second
	DefaultSevereThreshold           = 15 * time.Second
	DefaultInterval                  = 5 * time.Second
	ExtensionsDefaultTimeout         = extensionsbackupentry.DefaultTimeout
	ExtensionsDefaultInterval        = extensionsbackupentry.DefaultInterval
	ExtensionsDefaultSevereThreshold = extensionsbackupentry.DefaultSevereThreshold
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
	log             logr.Logger
	gardenClient    client.Client
	seedClient      client.Client
	backupBucket    *gardencorev1beta1.BackupBucket
	backupEntry     *gardencorev1beta1.BackupEntry
	component       extensionsbackupentry.Interface
	gardenNamespace string
}

func newActuator(log logr.Logger, gardenClient, seedClient client.Client, be *gardencorev1beta1.BackupEntry, clock clock.Clock, gardenNamespace string) Actuator {
	extensionSecret := emptyExtensionSecret(be, gardenNamespace)

	return &actuator{
		log:          log,
		gardenClient: gardenClient,
		seedClient:   seedClient,
		backupBucket: &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: be.Spec.BucketName,
			},
		},
		backupEntry: be,
		component: extensionsbackupentry.New(
			log,
			seedClient,
			clock,
			&extensionsbackupentry.Values{
				Name:       be.Name,
				BucketName: be.Spec.BucketName,
				SecretRef: corev1.SecretReference{
					Name:      extensionSecret.Name,
					Namespace: extensionSecret.Namespace,
				},
			},
			ExtensionsDefaultInterval,
			ExtensionsDefaultSevereThreshold,
			ExtensionsDefaultTimeout,
		),
		gardenNamespace: gardenNamespace,
	}
}

func (a *actuator) Reconcile(ctx context.Context) error {
	return nil
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
			Fn:           flow.TaskFn(a.deployBackupEntryExtensionSecret).RetryUntilTimeout(DefaultInterval, DefaultTimeout),
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
			Fn:           flow.TaskFn(a.deleteBackupEntryExtensionSecret).RetryUntilTimeout(DefaultInterval, DefaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilBackupEntryExtensionDeleted),
		})

		f = g.Compile()
	)

	return f.Run(ctx, flow.Opts{
		Log:              a.log,
		ProgressReporter: flow.NewImmediateProgressReporter(a.reportBackupEntryProgress),
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
			Fn:           flow.TaskFn(a.deleteBackupEntryExtensionSecret).RetryUntilTimeout(DefaultInterval, DefaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilBackupEntryExtensionDeleted),
		})

		f = g.Compile()
	)

	return f.Run(ctx, flow.Opts{
		Log:              a.log,
		ProgressReporter: flow.NewImmediateProgressReporter(a.reportBackupEntryProgress),
	})
}

// reportBackupEntryProgress will update the phase and error in the BackupEntry manifest `status` section
// by the current progress of the Flow execution.
func (a *actuator) reportBackupEntryProgress(ctx context.Context, stats *flow.Stats) {
	patch := client.MergeFrom(a.backupEntry.DeepCopy())

	if a.backupEntry.Status.LastOperation == nil {
		a.backupEntry.Status.LastOperation = &gardencorev1beta1.LastOperation{}
	}
	a.backupEntry.Status.LastOperation.Description = makeDescription(stats)
	a.backupEntry.Status.LastOperation.Progress = stats.ProgressPercent()
	a.backupEntry.Status.LastOperation.LastUpdateTime = metav1.Now()

	if err := a.gardenClient.Status().Patch(ctx, a.backupEntry, patch); err != nil {
		a.log.Error(err, "Could not report progress", "description", makeDescription(stats))
	}
}

func makeDescription(stats *flow.Stats) string {
	if stats.ProgressPercent() == 100 {
		return "Execution finished"
	}
	return strings.Join(stats.Running.StringList(), ", ")
}

// waitUntilBackupBucketReconciled waits until the BackupBucket in the garden cluster is reconciled.
func (a *actuator) waitUntilBackupBucketReconciled(ctx context.Context) error {
	if err := extensions.WaitUntilObjectReadyWithHealthFunction(
		ctx,
		a.gardenClient,
		a.log,
		health.CheckBackupBucket,
		a.backupBucket,
		"BackupBucket",
		DefaultInterval,
		DefaultSevereThreshold,
		DefaultTimeout,
		nil,
	); err != nil {
		return fmt.Errorf("associated BackupBucket %q is not ready yet with err: %w", a.backupEntry.Spec.BucketName, err)
	}

	return nil
}

func emptyExtensionSecret(backupEntry *gardencorev1beta1.BackupEntry, gardenNamespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("entry-%s", backupEntry.Name),
			Namespace: gardenNamespace,
		},
	}
}

func (a *actuator) deployBackupEntryExtensionSecret(ctx context.Context) error {
	coreSecretRef := &a.backupBucket.Spec.SecretRef
	if a.backupBucket.Status.GeneratedSecretRef != nil {
		coreSecretRef = a.backupBucket.Status.GeneratedSecretRef
	}

	coreSecret, err := kutil.GetSecretByReference(ctx, a.gardenClient, coreSecretRef)
	if err != nil {
		return fmt.Errorf("could not get secret referred in core backup bucket: %w", err)
	}

	// create secret for extension BackupEntry in seed
	extensionSecret := emptyExtensionSecret(a.backupEntry, a.gardenNamespace)
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, a.seedClient, extensionSecret, func() error {
		extensionSecret.Data = coreSecret.DeepCopy().Data
		return nil
	}); err != nil {
		return fmt.Errorf("could not reconcile extension secret in seed: %w", err)
	}

	return nil
}

// deleteBackupEntryExtensionSecret deletes secret referred by BackupEntry extension resource in seed.
func (a *actuator) deleteBackupEntryExtensionSecret(ctx context.Context) error {
	return client.IgnoreNotFound(a.seedClient.Delete(ctx, emptyExtensionSecret(a.backupEntry, a.gardenNamespace)))
}
