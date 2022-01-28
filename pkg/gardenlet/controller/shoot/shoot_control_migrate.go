// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"
	"errors"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/flow"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *shootReconciler) prepareShootForMigration(ctx context.Context, logger logrus.FieldLogger, gardenClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot, project *gardencorev1beta1.Project, cloudProfile *gardencorev1beta1.CloudProfile, seed *gardencorev1beta1.Seed) (reconcile.Result, error) {
	var (
		err error

		respectSyncPeriodOverwrite = r.respectSyncPeriodOverwrite()
		failed                     = gutil.IsShootFailed(shoot)
		ignored                    = gutil.ShouldIgnoreShoot(respectSyncPeriodOverwrite, shoot)
	)

	if failed || ignored {
		if syncErr := r.syncClusterResourceToSeed(ctx, shoot, project, cloudProfile, seed); syncErr != nil {
			logger.WithError(syncErr).Infof("Not allowed to update Shoot with error, trying to sync Cluster resource again")
			updateErr := r.patchShootStatusOperationError(ctx, gardenClient.Client(), shoot, syncErr.Error(), gardencorev1beta1.LastOperationTypeMigrate, shoot.Status.LastErrors...)
			return reconcile.Result{}, utilerrors.WithSuppressed(syncErr, updateErr)
		}
		logger.Info("Shoot is failed or ignored")
		return reconcile.Result{}, nil
	}

	r.recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventPrepareMigration, "Preparing Shoot cluster for migration")
	shootNamespace := shootpkg.ComputeTechnicalID(project.Name, shoot)
	if err = r.updateShootStatusOperationStart(ctx, gardenClient.Client(), shoot, shootNamespace, gardencorev1beta1.LastOperationTypeMigrate); err != nil {
		return reconcile.Result{}, err
	}

	o, operationErr := r.initializeOperation(ctx, logger, gardenClient, shoot, project, cloudProfile, seed)
	if operationErr != nil {
		updateErr := r.patchShootStatusOperationError(ctx, gardenClient.Client(), shoot, fmt.Sprintf("Could not initialize a new operation for Shoot cluster preparation for migration: %s", operationErr.Error()), gardencorev1beta1.LastOperationTypeMigrate, lastErrorsOperationInitializationFailure(shoot.Status.LastErrors, operationErr)...)
		return reconcile.Result{}, utilerrors.WithSuppressed(operationErr, updateErr)
	}
	// At this point the migration is allowed, hence, check if the seed is up-to-date, then sync the Cluster resource
	// initialize a new operation and, eventually, start the migration flow.
	if err := r.checkSeedAndSyncClusterResource(ctx, shoot, project, cloudProfile, seed); err != nil {
		return patchShootStatusAndRequeueOnSyncError(ctx, gardenClient.Client(), shoot, logger, err)
	}

	if flowErr := r.runPrepareShootForMigrationFlow(ctx, o); flowErr != nil {
		r.recorder.Event(shoot, corev1.EventTypeWarning, gardencorev1beta1.EventMigrationPreparationFailed, flowErr.Description)
		updateErr := r.patchShootStatusOperationError(ctx, gardenClient.Client(), shoot, flowErr.Description, gardencorev1beta1.LastOperationTypeMigrate, flowErr.LastErrors...)
		return reconcile.Result{}, utilerrors.WithSuppressed(errors.New(flowErr.Description), updateErr)
	}

	return r.finalizeShootPrepareForMigration(ctx, gardenClient.Client(), shoot, o)
}

func (r *shootReconciler) runPrepareShootForMigrationFlow(ctx context.Context, o *operation.Operation) *gardencorev1beta1helper.WrappedLastErrors {
	var (
		botanist                      *botanistpkg.Botanist
		err                           error
		tasksWithErrors               []string
		controlPlaneRestorationNeeded bool
		infrastructure                *extensionsv1alpha1.Infrastructure
		kubeAPIServerDeploymentFound  = true
		etcdSnapshotRequired          bool
	)

	for _, lastError := range o.Shoot.GetInfo().Status.LastErrors {
		if lastError.TaskID != nil {
			tasksWithErrors = append(tasksWithErrors, *lastError.TaskID)
		}
	}

	errorContext := utilerrors.NewErrorContext("Shoot cluster preparation for migration", tasksWithErrors)

	err = utilerrors.HandleErrors(errorContext,
		func(errorID string) error {
			o.CleanShootTaskError(ctx, errorID)
			return nil
		},
		nil,
		utilerrors.ToExecute("Create botanist", func() error {
			return retryutils.UntilTimeout(ctx, 10*time.Second, 10*time.Minute, func(context.Context) (done bool, err error) {
				botanist, err = botanistpkg.New(ctx, o)
				if err != nil {
					return retryutils.MinorError(err)
				}
				return retryutils.Ok()
			})
		}),
		utilerrors.ToExecute("Retrieve kube-apiserver deployment in the shoot namespace in the seed cluster", func() error {
			deploymentKubeAPIServer := &appsv1.Deployment{}
			if err := botanist.K8sSeedClient.APIReader().Get(ctx, kutil.Key(o.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer), deploymentKubeAPIServer); err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
				kubeAPIServerDeploymentFound = false
			}
			if deploymentKubeAPIServer.DeletionTimestamp != nil {
				kubeAPIServerDeploymentFound = false
			}
			return nil
		}),
		utilerrors.ToExecute("Retrieve the Shoot namespace in the Seed cluster", func() error {
			botanist.SeedNamespaceObject = &corev1.Namespace{}
			err := botanist.K8sSeedClient.APIReader().Get(ctx, client.ObjectKey{Name: o.Shoot.SeedNamespace}, botanist.SeedNamespaceObject)
			if err != nil {
				if apierrors.IsNotFound(err) {
					o.Logger.Infof("Did not find '%s' namespace in the Seed cluster - nothing to be done", o.Shoot.SeedNamespace)
					return utilerrors.Cancel()
				}
			}
			return err
		}),
		utilerrors.ToExecute("Retrieve the BackupEntry in the garden cluster", func() error {
			backupentry := &gardencorev1beta1.BackupEntry{}
			err := botanist.K8sGardenClient.APIReader().Get(ctx, client.ObjectKey{Name: botanist.Shoot.BackupEntryName, Namespace: o.Shoot.GetInfo().Namespace}, backupentry)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
				return nil
			}
			etcdSnapshotRequired = backupentry.Spec.SeedName != nil && *backupentry.Spec.SeedName == *botanist.Shoot.GetInfo().Status.SeedName
			return nil
		}),
		utilerrors.ToExecute("Retrieve the infrastructure resource", func() error {
			obj, err := botanist.Shoot.Components.Extensions.Infrastructure.Get(ctx)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			infrastructure = obj
			return nil
		}),
		utilerrors.ToExecute("Check whether control plane restoration is needed", func() error {
			controlPlaneRestorationNeeded, err = needsControlPlaneDeployment(ctx, o, kubeAPIServerDeploymentFound, infrastructure)
			return err
		}),
	)

	if err != nil {
		if utilerrors.WasCanceled(err) {
			return nil
		}
		return gardencorev1beta1helper.NewWrappedLastErrors(gardencorev1beta1helper.FormatLastErrDescription(err), err)
	}

	var (
		nonTerminatingNamespace = botanist.SeedNamespaceObject.Status.Phase != corev1.NamespaceTerminating
		cleanupShootResources   = nonTerminatingNamespace && kubeAPIServerDeploymentFound
		wakeupRequired          = (o.Shoot.GetInfo().Status.IsHibernated || o.Shoot.HibernationEnabled) && cleanupShootResources
		defaultTimeout          = 10 * time.Minute
		defaultInterval         = 5 * time.Second

		g = flow.NewGraph("Shoot cluster preparation for migration")

		ensureShootStateExists = g.Add(flow.Task{
			Name: "Ensuring that ShootState exists",
			Fn:   flow.TaskFn(botanist.EnsureShootStateExists).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		generateSecrets = g.Add(flow.Task{
			Name:         "Generating secrets and saving them into ShootState",
			Fn:           botanist.GenerateAndSaveSecrets,
			Dependencies: flow.NewTaskIDs(ensureShootStateExists),
		})
		deploySecrets = g.Add(flow.Task{
			Name:         "Deploying Shoot certificates / keys",
			Fn:           flow.TaskFn(botanist.DeploySecrets).DoIf(nonTerminatingNamespace),
			Dependencies: flow.NewTaskIDs(ensureShootStateExists, generateSecrets),
		})
		deployETCD = g.Add(flow.Task{
			Name:         "Deploying main and events etcd",
			Fn:           flow.TaskFn(botanist.DeployEtcd).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(cleanupShootResources || etcdSnapshotRequired),
			Dependencies: flow.NewTaskIDs(deploySecrets),
		})
		scaleETCDToOne = g.Add(flow.Task{
			Name:         "Scaling etcd up",
			Fn:           flow.TaskFn(botanist.ScaleETCDToOne).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(wakeupRequired),
			Dependencies: flow.NewTaskIDs(deployETCD),
		})
		waitUntilEtcdReady = g.Add(flow.Task{
			Name:         "Waiting until main and event etcd report readiness",
			Fn:           flow.TaskFn(botanist.WaitUntilEtcdsReady).DoIf(cleanupShootResources || etcdSnapshotRequired),
			Dependencies: flow.NewTaskIDs(deployETCD, scaleETCDToOne),
		})
		generateEncryptionConfigurationMetaData = g.Add(flow.Task{
			Name:         "Generating etcd encryption configuration",
			Fn:           flow.TaskFn(botanist.GenerateEncryptionConfiguration).RetryUntilTimeout(defaultInterval, defaultTimeout).DoIf(wakeupRequired),
			Dependencies: flow.NewTaskIDs(ensureShootStateExists),
		})
		persistETCDEncryptionConfiguration = g.Add(flow.Task{
			Name:         "Persisting etcd encryption configuration in ShootState",
			Fn:           flow.TaskFn(botanist.PersistEncryptionConfiguration).DoIf(wakeupRequired),
			Dependencies: flow.NewTaskIDs(generateEncryptionConfigurationMetaData),
		})
		applyETCDEncryptionConfiguration = g.Add(flow.Task{
			Name:         "Applying etcd encryption configuration",
			Fn:           flow.TaskFn(botanist.ApplyEncryptionConfiguration).DoIf(wakeupRequired),
			Dependencies: flow.NewTaskIDs(persistETCDEncryptionConfiguration),
		})
		// Restore the control plane in case it was already migrated to make sure all components that depend on the cloud provider secret are restarted
		// in case it has changed. Also, it's needed for other control plane components like the kube-apiserver or kube-
		// controller-manager to be updateable due to provider config injection.
		restoreControlPlane = g.Add(flow.Task{
			Name:         "Restoring Shoot control plane",
			Fn:           flow.TaskFn(botanist.RestoreControlPlane).DoIf(cleanupShootResources && controlPlaneRestorationNeeded),
			Dependencies: flow.NewTaskIDs(deploySecrets),
		})
		waitUntilControlPlaneReady = g.Add(flow.Task{
			Name:         "Waiting until Shoot control plane has been restored",
			Fn:           flow.TaskFn(botanist.Shoot.Components.Extensions.ControlPlane.Wait).DoIf(cleanupShootResources && controlPlaneRestorationNeeded),
			Dependencies: flow.NewTaskIDs(restoreControlPlane),
		})
		wakeUpKubeAPIServer = g.Add(flow.Task{
			Name:         "Scaling Kubernetes API Server up and waiting until ready",
			Fn:           flow.TaskFn(botanist.WakeUpKubeAPIServer).DoIf(wakeupRequired),
			Dependencies: flow.NewTaskIDs(deployETCD, scaleETCDToOne, applyETCDEncryptionConfiguration, waitUntilControlPlaneReady),
		})
		ensureResourceManagerScaledUp = g.Add(flow.Task{
			Name:         "Ensuring that the gardener resource manager is scaled to 1",
			Fn:           flow.TaskFn(botanist.ScaleGardenerResourceManagerToOne).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(wakeUpKubeAPIServer),
		})
		keepManagedResourcesObjectsInShoot = g.Add(flow.Task{
			Name:         "Configuring Managed Resources objects to be kept in the Shoot",
			Fn:           flow.TaskFn(botanist.KeepObjectsForManagedResources).DoIf(cleanupShootResources),
			Dependencies: flow.NewTaskIDs(ensureResourceManagerScaledUp),
		})
		deleteManagedResources = g.Add(flow.Task{
			Name:         "Deleting all Managed Resources from the Shoot's namespace",
			Fn:           flow.TaskFn(botanist.DeleteManagedResources),
			Dependencies: flow.NewTaskIDs(keepManagedResourcesObjectsInShoot, ensureResourceManagerScaledUp),
		})
		waitForManagedResourcesDeletion = g.Add(flow.Task{
			Name:         "Waiting until ManagedResources are deleted",
			Fn:           flow.TaskFn(botanist.WaitUntilManagedResourcesDeleted).Timeout(10 * time.Minute),
			Dependencies: flow.NewTaskIDs(deleteManagedResources),
		})
		migrateExtensionResources = g.Add(flow.Task{
			Name:         "Migrating extension resources",
			Fn:           botanist.MigrateExtensionResourcesInParallel,
			Dependencies: flow.NewTaskIDs(waitForManagedResourcesDeletion),
		})
		waitUntilExtensionResourcesMigrated = g.Add(flow.Task{
			Name:         "Waiting until extension resources have been migrated",
			Fn:           botanist.WaitUntilExtensionResourcesMigrated,
			Dependencies: flow.NewTaskIDs(migrateExtensionResources),
		})
		deleteExtensionResources = g.Add(flow.Task{
			Name:         "Deleting extension resources from the Shoot namespace",
			Fn:           botanist.DestroyExtensionResourcesInParallel,
			Dependencies: flow.NewTaskIDs(waitUntilExtensionResourcesMigrated),
		})
		waitUntilExtensionResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension resources have been deleted",
			Fn:           botanist.WaitUntilExtensionResourcesDeleted,
			Dependencies: flow.NewTaskIDs(deleteExtensionResources),
		})
		migrateControlPlane = g.Add(flow.Task{
			Name:         "Migrating shoot control plane",
			Fn:           botanist.Shoot.Components.Extensions.ControlPlane.Migrate,
			Dependencies: flow.NewTaskIDs(waitUntilExtensionResourcesDeleted),
		})
		deleteControlPlane = g.Add(flow.Task{
			Name:         "Deleting shoot control plane",
			Fn:           flow.TaskFn(botanist.Shoot.Components.Extensions.ControlPlane.Destroy).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(migrateControlPlane),
		})
		waitUntilControlPlaneDeleted = g.Add(flow.Task{
			Name:         "Waiting until shoot control plane has been deleted",
			Fn:           botanist.Shoot.Components.Extensions.ControlPlane.WaitCleanup,
			Dependencies: flow.NewTaskIDs(deleteControlPlane),
		})
		deleteKubeAPIServer = g.Add(flow.Task{
			Name:         "Deleting kube-apiserver deployment",
			Fn:           flow.TaskFn(botanist.DeleteKubeAPIServer).SkipIf(!kubeAPIServerDeploymentFound),
			Dependencies: flow.NewTaskIDs(waitForManagedResourcesDeletion, waitUntilEtcdReady, waitUntilControlPlaneDeleted),
		})
		waitUntilKubeAPIServerDeleted = g.Add(flow.Task{
			Name:         "Waiting until kube-apiserver has been deleted",
			Fn:           flow.TaskFn(botanist.Shoot.Components.ControlPlane.KubeAPIServer.WaitCleanup),
			Dependencies: flow.NewTaskIDs(deleteKubeAPIServer),
		})
		migrateInfrastructure = g.Add(flow.Task{
			Name:         "Migrating shoot infrastructure",
			Fn:           botanist.Shoot.Components.Extensions.Infrastructure.Migrate,
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		waitUntilInfrastructureMigrated = g.Add(flow.Task{
			Name:         "Waiting until shoot infrastructure has been migrated",
			Fn:           botanist.Shoot.Components.Extensions.Infrastructure.WaitMigrate,
			Dependencies: flow.NewTaskIDs(migrateInfrastructure),
		})
		_ = g.Add(flow.Task{
			Name:         "Deleting shoot infrastructure",
			Fn:           botanist.Shoot.Components.Extensions.Infrastructure.Destroy,
			Dependencies: flow.NewTaskIDs(waitUntilInfrastructureMigrated),
		})
		migrateIngressDNSRecord = g.Add(flow.Task{
			Name:         "Migrating nginx ingress DNS record",
			Fn:           flow.TaskFn(botanist.MigrateIngressDNSResources),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		migrateExternalDNSRecord = g.Add(flow.Task{
			Name:         "Migrating external domain DNS record",
			Fn:           flow.TaskFn(botanist.MigrateExternalDNSResources),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		migrateInternalDNSRecord = g.Add(flow.Task{
			Name:         "Migrating internal domain DNS record",
			Fn:           flow.TaskFn(botanist.MigrateInternalDNSResources),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		migrateOwnerDNSRecord = g.Add(flow.Task{
			Name:         "Migrating owner domain DNS record",
			Fn:           flow.TaskFn(botanist.MigrateOwnerDNSResources),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		destroyDNSRecords = g.Add(flow.Task{
			Name:         "Deleting DNSRecords from the Shoot namespace",
			Fn:           botanist.DestroyDNSRecords,
			Dependencies: flow.NewTaskIDs(migrateIngressDNSRecord, migrateExternalDNSRecord, migrateInternalDNSRecord, migrateOwnerDNSRecord),
		})
		destroyDNSProviders = g.Add(flow.Task{
			Name:         "Deleting DNS providers",
			Fn:           botanist.DeleteDNSProviders,
			Dependencies: flow.NewTaskIDs(migrateIngressDNSRecord, migrateExternalDNSRecord, migrateInternalDNSRecord, migrateOwnerDNSRecord),
		})
		createETCDSnapshot = g.Add(flow.Task{
			Name:         "Creating ETCD Snapshot",
			Fn:           flow.TaskFn(botanist.SnapshotEtcd).DoIf(etcdSnapshotRequired),
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		migrateBackupEntryInGarden = g.Add(flow.Task{
			Name:         "Migrating BackupEntry to new seed",
			Fn:           botanist.Shoot.Components.BackupEntry.Migrate,
			Dependencies: flow.NewTaskIDs(createETCDSnapshot),
		})
		waitUntilBackupEntryInGardenMigrated = g.Add(flow.Task{
			Name:         "Waiting for BackupEntry to be migrated to new seed",
			Fn:           botanist.Shoot.Components.BackupEntry.WaitMigrate,
			Dependencies: flow.NewTaskIDs(migrateBackupEntryInGarden),
		})
		destroyEtcd = g.Add(flow.Task{
			Name:         "Destroying main and events etcd",
			Fn:           flow.TaskFn(botanist.DestroyEtcd).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(createETCDSnapshot, waitUntilBackupEntryInGardenMigrated),
		})
		waitUntilEtcdDeleted = g.Add(flow.Task{
			Name:         "Waiting until main and event etcd have been destroyed",
			Fn:           flow.TaskFn(botanist.WaitUntilEtcdsDeleted).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(destroyEtcd),
		})
		deleteNamespace = g.Add(flow.Task{
			Name:         "Deleting shoot namespace in Seed",
			Fn:           flow.TaskFn(botanist.DeleteSeedNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitUntilBackupEntryInGardenMigrated, deleteExtensionResources, destroyDNSRecords, destroyDNSProviders, waitForManagedResourcesDeletion, waitUntilEtcdDeleted),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until shoot namespace in Seed has been deleted",
			Fn:           botanist.WaitUntilSeedNamespaceDeleted,
			Dependencies: flow.NewTaskIDs(deleteNamespace),
		})

		f = g.Compile()
	)

	if err := f.Run(ctx, flow.Opts{
		Logger:           o.Logger,
		ProgressReporter: r.newProgressReporter(o.ReportShootProgress),
		ErrorContext:     errorContext,
		ErrorCleaner:     o.CleanShootTaskError,
	}); err != nil {
		o.Logger.Errorf("Failed to prepare Shoot cluster %q for migration: %+v", o.Shoot.GetInfo().Name, err)
		return gardencorev1beta1helper.NewWrappedLastErrors(gardencorev1beta1helper.FormatLastErrDescription(err), flow.Errors(err))
	}

	o.Logger.Infof("Successfully prepared Shoot cluster %q for migration", o.Shoot.GetInfo().Name)
	return nil
}

func (r *shootReconciler) finalizeShootPrepareForMigration(ctx context.Context, gardenClient client.Client, shoot *gardencorev1beta1.Shoot, o *operation.Operation) (reconcile.Result, error) {
	if len(shoot.Status.UID) > 0 {
		if err := o.DeleteClusterResourceFromSeed(ctx); err != nil {
			lastErr := gardencorev1beta1helper.LastError(fmt.Sprintf("Could not delete Cluster resource in seed: %s", err))
			r.recorder.Event(shoot, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, lastErr.Description)
			updateErr := r.patchShootStatusOperationError(ctx, gardenClient, shoot, lastErr.Description, gardencorev1beta1.LastOperationTypeMigrate, *lastErr)
			return reconcile.Result{}, utilerrors.WithSuppressed(errors.New(lastErr.Description), updateErr)
		}
	}

	metaPatch := client.MergeFrom(shoot.DeepCopy())
	controllerutils.RemoveAllTasks(shoot.Annotations)
	if err := gardenClient.Patch(ctx, shoot, metaPatch); err != nil {
		return reconcile.Result{}, err
	}

	r.recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.EventMigrationPrepared, "Prepared Shoot cluster for migration")
	if err := r.patchShootStatusOperationSuccess(ctx, gardenClient, shoot, o.Shoot.SeedNamespace, nil, gardencorev1beta1.LastOperationTypeMigrate); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
