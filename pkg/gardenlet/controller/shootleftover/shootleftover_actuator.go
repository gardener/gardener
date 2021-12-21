// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shootleftover

import (
	"context"
	"fmt"
	"strings"
	"time"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/logger"
	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/flow"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Actuator acts upon ShootLeftover resources.
type Actuator interface {
	// Reconcile reconciles ShootLeftover creation or update.
	Reconcile(context.Context, *gardencorev1alpha1.ShootLeftover) (bool, error)
	// Delete reconciles ShootLeftover deletion.
	Delete(context.Context, *gardencorev1alpha1.ShootLeftover) (bool, error)
}

// actuator is a concrete implementation of Actuator.
type actuator struct {
	gardenClient kubernetes.Interface
	clientMap    clientmap.ClientMap
}

// newActuator creates a new Actuator with the given clients and logger.
func newActuator(gardenClient kubernetes.Interface, clientMap clientmap.ClientMap) Actuator {
	return &actuator{
		gardenClient: gardenClient,
		clientMap:    clientMap,
	}
}

// Reconcile reconciles ShootLeftover creation or update.
func (a *actuator) Reconcile(ctx context.Context, slo *gardencorev1alpha1.ShootLeftover) (bool, error) {
	// Get seed client
	seedClient, err := a.clientMap.GetClient(ctx, keys.ForSeedWithName(slo.Spec.SeedName))
	if err != nil {
		return false, fmt.Errorf("could not get seed client for seed %s: %w", slo.Spec.SeedName, err)
	}

	var (
		namespace   *corev1.Namespace
		cluster     *extensionsv1alpha1.Cluster
		backupEntry *extensionsv1alpha1.BackupEntry
		dnsOwners   []dnsv1alpha1.DNSOwner

		defaultInterval = 5 * time.Second
		defaultTimeout  = 30 * time.Second

		cleaner = newCleaner(seedClient.Client(), *slo.Spec.TechnicalID, string(*slo.Spec.UID), logf.FromContext(ctx), a.getFieldLogger(slo))

		errorContext = utilerrors.NewErrorContext("Shoot leftover resources reconciliation", gardencorev1beta1helper.GetTaskIDs(slo.Status.LastErrors))

		g = flow.NewGraph("Shoot leftover resources reconciliation")

		_ = g.Add(flow.Task{
			Name: "Checking if shoot namespace exists",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				var err error
				namespace, err = cleaner.GetNamespace(ctx)
				return err
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		_ = g.Add(flow.Task{
			Name: "Checking if Cluster resource exists",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				var err error
				cluster, err = cleaner.GetCluster(ctx)
				return err
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		_ = g.Add(flow.Task{
			Name: "Checking if BackupEntry resource exists",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				var err error
				backupEntry, err = cleaner.GetBackupEntry(ctx)
				return err
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		_ = g.Add(flow.Task{
			Name: "Checking if DNSOwner resources exists",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				var err error
				dnsOwners, err = cleaner.GetDNSOwners(ctx)
				return err
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})

		f = g.Compile()
	)

	err = f.Run(ctx, flow.Opts{
		Logger:           a.getFieldLogger(slo),
		ProgressReporter: flow.NewImmediateProgressReporter(a.getProgressReporterFunc(slo)),
		ErrorCleaner:     a.getErrorCleanerFunc(slo),
		ErrorContext:     errorContext,
	})

	return namespace != nil || cluster != nil || backupEntry != nil || len(dnsOwners) > 0, err
}

// Delete reconciles ShootLeftover deletion.
func (a *actuator) Delete(ctx context.Context, slo *gardencorev1alpha1.ShootLeftover) (bool, error) {
	// Get seed client
	seedClient, err := a.clientMap.GetClient(ctx, keys.ForSeedWithName(slo.Spec.SeedName))
	if err != nil {
		return false, fmt.Errorf("could not get seed client for seed %s: %w", slo.Spec.SeedName, err)
	}

	var (
		defaultInterval = 5 * time.Second
		defaultTimeout  = 5 * time.Minute

		cleaner = newCleaner(seedClient.Client(), *slo.Spec.TechnicalID, string(*slo.Spec.UID), logf.FromContext(ctx), a.getFieldLogger(slo))

		errorContext = utilerrors.NewErrorContext("Shoot leftover resources deletion", gardencorev1beta1helper.GetTaskIDs(slo.Status.LastErrors))

		g = flow.NewGraph("Shoot leftover resources deletion")

		migrateExtensionObjects = g.Add(flow.Task{
			Name:         "Migrating extension resources",
			Fn:           flow.TaskFn(cleaner.MigrateExtensionObjects).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(),
		})
		waitUntilExtensionObjectsMigrated = g.Add(flow.Task{
			Name:         "Waiting until extension resources have been migrated",
			Fn:           cleaner.WaitUntilExtensionObjectsMigrated,
			Dependencies: flow.NewTaskIDs(migrateExtensionObjects),
		})
		deleteExtensionObjects = g.Add(flow.Task{
			Name:         "Deleting extension resources",
			Fn:           flow.TaskFn(cleaner.DeleteExtensionObjects).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilExtensionObjectsMigrated),
		})
		waitUntilExtensionObjectsDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension resources have been deleted",
			Fn:           cleaner.WaitUntilExtensionObjectsDeleted,
			Dependencies: flow.NewTaskIDs(deleteExtensionObjects),
		})
		migrateBackupEntry = g.Add(flow.Task{
			Name:         "Migrating BackupEntry resource",
			Fn:           flow.TaskFn(cleaner.MigrateBackupEntry).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(),
		})
		waitUntilBackupEntryMigrated = g.Add(flow.Task{
			Name:         "Waiting until BackupEntry resource has been migrated",
			Fn:           cleaner.WaitUntilBackupEntryMigrated,
			Dependencies: flow.NewTaskIDs(migrateBackupEntry),
		})
		deleteBackupEntry = g.Add(flow.Task{
			Name:         "Deleting BackupEntry resource",
			Fn:           flow.TaskFn(cleaner.DeleteBackupEntry).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilBackupEntryMigrated),
		})
		waitUntilBackupEntryDeleted = g.Add(flow.Task{
			Name:         "Waiting until BackupEntry resource has been deleted",
			Fn:           cleaner.WaitUntilBackupEntryDeleted,
			Dependencies: flow.NewTaskIDs(deleteBackupEntry),
		})
		deleteCluster = g.Add(flow.Task{
			Name:         "Deleting Cluster resource",
			Fn:           flow.TaskFn(cleaner.DeleteCluster).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilExtensionObjectsDeleted, waitUntilBackupEntryDeleted),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until Cluster resource has been deleted",
			Fn:           cleaner.WaitUntilClusterDeleted,
			Dependencies: flow.NewTaskIDs(deleteCluster),
		})
		deleteEtcds = g.Add(flow.Task{
			Name:         "Deleting Etcd resources",
			Fn:           flow.TaskFn(cleaner.DeleteEtcds).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(),
		})
		waitUntilEtcdsDeleted = g.Add(flow.Task{
			Name:         "Waiting until Etcd resources have been deleted",
			Fn:           cleaner.WaitUntilEtcdsDeleted,
			Dependencies: flow.NewTaskIDs(deleteEtcds),
		})
		setKeepObjectsForManagedResources = g.Add(flow.Task{
			Name:         "Configuring managed resources to keep their objects when deleted",
			Fn:           flow.TaskFn(cleaner.SetKeepObjectsForManagedResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(),
		})
		deleteManagedResources = g.Add(flow.Task{
			Name:         "Deleting managed resources",
			Fn:           flow.TaskFn(cleaner.DeleteManagedResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(setKeepObjectsForManagedResources),
		})
		waitUntilManagedResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until managed resources have been deleted",
			Fn:           cleaner.WaitUntilManagedResourcesDeleted,
			Dependencies: flow.NewTaskIDs(deleteManagedResources),
		})
		deleteDNSOwners = g.Add(flow.Task{
			Name:         "Deleting DNSOwner resources",
			Fn:           flow.TaskFn(cleaner.DeleteDNSOwners).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(),
		})
		waitUntilDNSOwnersDeleted = g.Add(flow.Task{
			Name:         "Waiting until DNSOwner resources have been deleted",
			Fn:           cleaner.WaitUntilDNSOwnersDeleted,
			Dependencies: flow.NewTaskIDs(deleteDNSOwners),
		})
		deleteDNSEntries = g.Add(flow.Task{
			Name:         "Deleting DNSEntry resources",
			Fn:           flow.TaskFn(cleaner.DeleteDNSEntries).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilDNSOwnersDeleted),
		})
		waitUntilDNSEntriesDeleted = g.Add(flow.Task{
			Name:         "Waiting until DNSEntry resources have been deleted",
			Fn:           cleaner.WaitUntilDNSEntriesDeleted,
			Dependencies: flow.NewTaskIDs(deleteDNSEntries),
		})
		deleteDNSProviders = g.Add(flow.Task{
			Name:         "Deleting DNSProvider resources",
			Fn:           flow.TaskFn(cleaner.DeleteDNSProviders).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilDNSEntriesDeleted),
		})
		waitUntilDNSProvidersDeleted = g.Add(flow.Task{
			Name:         "Waiting until DNSProvider resources have been deleted",
			Fn:           cleaner.WaitUntilDNSProvidersDeleted,
			Dependencies: flow.NewTaskIDs(deleteDNSProviders),
		})
		deleteSecrets = g.Add(flow.Task{
			Name:         "Deleting secrets",
			Fn:           flow.TaskFn(cleaner.DeleteSecrets).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilExtensionObjectsDeleted, waitUntilEtcdsDeleted, waitUntilManagedResourcesDeleted, waitUntilDNSEntriesDeleted, waitUntilDNSProvidersDeleted),
		})
		deleteNamespace = g.Add(flow.Task{
			Name:         "Deleting shoot namespace",
			Fn:           flow.TaskFn(cleaner.DeleteNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilExtensionObjectsDeleted, waitUntilEtcdsDeleted, waitUntilManagedResourcesDeleted, waitUntilDNSEntriesDeleted, waitUntilDNSProvidersDeleted, deleteSecrets),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until shoot namespace has been deleted",
			Fn:           cleaner.WaitUntilNamespaceDeleted,
			Dependencies: flow.NewTaskIDs(deleteNamespace),
		})

		f = g.Compile()
	)

	err = f.Run(ctx, flow.Opts{
		Logger:           a.getFieldLogger(slo),
		ProgressReporter: flow.NewImmediateProgressReporter(a.getProgressReporterFunc(slo)),
		ErrorCleaner:     a.getErrorCleanerFunc(slo),
		ErrorContext:     errorContext,
	})

	return err != nil, err
}

// getFieldLogger returns a logrus.FieldLogger for the given ShootLeftover object with exactly the same fields
// as the configured logr.Logger.
// TODO Remove when logrus loggers are no longer used by flow and other library packages
func (a *actuator) getFieldLogger(slo *gardencorev1alpha1.ShootLeftover) logrus.FieldLogger {
	return logger.Logger.WithFields(logrus.Fields{"logger": "controller." + ControllerName, "name": slo.Name, "namespace": slo.Namespace})
}

func (a *actuator) getProgressReporterFunc(slo *gardencorev1alpha1.ShootLeftover) func(ctx context.Context, stats *flow.Stats) {
	return func(ctx context.Context, stats *flow.Stats) {
		patch := client.StrategicMergeFrom(slo.DeepCopy())

		slo.Status.LastOperation.Description = strings.Join(stats.Running.StringList(), ", ")
		slo.Status.LastOperation.Progress = stats.ProgressPercent()
		slo.Status.LastOperation.LastUpdateTime = metav1.Now()

		if err := a.gardenClient.Client().Status().Patch(ctx, slo, patch); err != nil {
			logf.FromContext(ctx).Error(err, "Could not report progress")
		}
	}
}

func (a *actuator) getErrorCleanerFunc(slo *gardencorev1alpha1.ShootLeftover) func(ctx context.Context, taskID string) {
	return func(ctx context.Context, taskID string) {
		patch := client.StrategicMergeFrom(slo.DeepCopy())

		slo.Status.LastErrors = gardencorev1beta1helper.DeleteLastErrorByTaskID(slo.Status.LastErrors, taskID)

		if err := a.gardenClient.Client().Status().Patch(ctx, slo, patch); err != nil {
			logf.FromContext(ctx).Error(err, "Could not update last errors")
		}
	}
}
