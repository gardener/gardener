// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/flow"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
)

func (r *Reconciler) runMigrateShootFlow(ctx context.Context, o *operation.Operation) *v1beta1helper.WrappedLastErrors {
	var (
		botanist                     *botanistpkg.Botanist
		err                          error
		tasksWithErrors              []string
		kubeAPIServerDeploymentFound = true
		etcdSnapshotRequired         bool
	)

	for _, lastError := range o.Shoot.GetInfo().Status.LastErrors {
		if lastError.TaskID != nil {
			tasksWithErrors = append(tasksWithErrors, *lastError.TaskID)
		}
	}

	errorContext := errorsutils.NewErrorContext("Shoot cluster preparation for migration", tasksWithErrors)

	err = errorsutils.HandleErrors(errorContext,
		func(errorID string) error {
			o.CleanShootTaskError(ctx, errorID)
			return nil
		},
		nil,
		errorsutils.ToExecute("Create botanist", func() error {
			return retryutils.UntilTimeout(ctx, 10*time.Second, 10*time.Minute, func(context.Context) (done bool, err error) {
				botanist, err = botanistpkg.New(ctx, o)
				if err != nil {
					return retryutils.MinorError(err)
				}
				return retryutils.Ok()
			})
		}),
		errorsutils.ToExecute("Retrieve kube-apiserver deployment in the shoot namespace in the seed cluster", func() error {
			deploymentKubeAPIServer := &appsv1.Deployment{}
			if err := botanist.SeedClientSet.APIReader().Get(ctx, client.ObjectKey{Namespace: o.Shoot.ControlPlaneNamespace, Name: v1beta1constants.DeploymentNameKubeAPIServer}, deploymentKubeAPIServer); err != nil {
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
		errorsutils.ToExecute("Retrieve the Shoot namespace in the Seed cluster", func() error {
			return checkIfSeedNamespaceExists(ctx, o, botanist)
		}),
		errorsutils.ToExecute("Retrieve the BackupEntry in the garden cluster", func() error {
			backupEntry := &gardencorev1beta1.BackupEntry{}
			err := botanist.GardenClient.Get(ctx, client.ObjectKey{Name: botanist.Shoot.BackupEntryName, Namespace: o.Shoot.GetInfo().Namespace}, backupEntry)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
				return nil
			}
			etcdSnapshotRequired = backupEntry.Spec.SeedName != nil && *backupEntry.Spec.SeedName == *botanist.Shoot.GetInfo().Status.SeedName && botanist.SeedNamespaceObject.UID != ""
			return nil
		}),
	)
	if err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
	}

	const (
		defaultTimeout  = 10 * time.Minute
		defaultInterval = 5 * time.Second
	)

	var (
		hasNodesCIDR            = o.Shoot.GetInfo().Spec.Networking != nil && o.Shoot.GetInfo().Spec.Networking.Nodes != nil
		nonTerminatingNamespace = botanist.SeedNamespaceObject.UID != "" && botanist.SeedNamespaceObject.Status.Phase != corev1.NamespaceTerminating
		cleanupShootResources   = nonTerminatingNamespace && kubeAPIServerDeploymentFound
		wakeupRequired          = (o.Shoot.GetInfo().Status.IsHibernated || o.Shoot.HibernationEnabled) && cleanupShootResources
	)

	if hasNodesCIDR {
		networks, err := shoot.ToNetworks(o.Shoot.GetInfo(), o.Shoot.IsWorkerless)
		if err != nil {
			return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
		}
		o.Shoot.Networks = networks
	}

	var (
		g = flow.NewGraph("Shoot cluster preparation for migration")

		deployNamespace = g.Add(flow.Task{
			Name:   "Deploying Shoot namespace in Seed",
			Fn:     flow.TaskFn(botanist.DeployControlPlaneNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: !nonTerminatingNamespace,
		})
		reconcileIstioInternalLoadbalancingConfigMap = g.Add(flow.Task{
			Name:         "Reconcile Istio internal load balancing ConfigMap",
			Fn:           flow.TaskFn(botanist.ReconcileIstioInternalLoadBalancingConfigMap).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       !nonTerminatingNamespace,
			Dependencies: flow.NewTaskIDs(deployNamespace),
		})
		initializeSecretsManagement = g.Add(flow.Task{
			Name:         "Initializing secrets management",
			Fn:           flow.TaskFn(botanist.InitializeSecretsManagement).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       !nonTerminatingNamespace,
			Dependencies: flow.NewTaskIDs(deployNamespace, reconcileIstioInternalLoadbalancingConfigMap),
		})
		deployETCD = g.Add(flow.Task{
			Name:         "Deploying main and events etcd",
			Fn:           flow.TaskFn(botanist.DeployEtcd).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       !cleanupShootResources && !etcdSnapshotRequired,
			Dependencies: flow.NewTaskIDs(initializeSecretsManagement),
		})
		scaleUpETCD = g.Add(flow.Task{
			Name:         "Scaling etcd up",
			Fn:           flow.TaskFn(botanist.ScaleUpETCD).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       !wakeupRequired,
			Dependencies: flow.NewTaskIDs(deployETCD),
		})
		waitUntilEtcdReady = g.Add(flow.Task{
			Name:         "Waiting until main and event etcd report readiness",
			Fn:           botanist.WaitUntilEtcdsReady,
			SkipIf:       !cleanupShootResources && !etcdSnapshotRequired,
			Dependencies: flow.NewTaskIDs(deployETCD, scaleUpETCD),
		})
		wakeUpKubeAPIServer = g.Add(flow.Task{
			Name:         "Scaling Kubernetes API Server up and waiting until ready",
			Fn:           botanist.WakeUpKubeAPIServer,
			SkipIf:       !wakeupRequired,
			Dependencies: flow.NewTaskIDs(deployETCD, scaleUpETCD, initializeSecretsManagement),
		})
		migrateManagedResources                   = g.AddGroup(botanist.MigrateAndDeleteManagedResourcesTaskGroup(cleanupShootResources).WithDependencies(wakeUpKubeAPIServer))
		migrateExtensionResourcesGroup            = g.AddGroup(botanist.MigrateExtensionResourcesTaskGroup().WithDependencies(migrateManagedResources))
		migrateExtensionsBeforeKubeAPIServerGroup = g.AddGroup(botanist.MigrateExtensionsBeforeKubeAPIServerTaskGroup().WithDependencies(migrateExtensionResourcesGroup))
		destroyExtensionAndMachineResourcesGroup  = g.AddGroup(botanist.DestroyExtensionAndMachineResourcesTaskGroup().WithDependencies(migrateExtensionsBeforeKubeAPIServerGroup))
		migrateControlPlane                       = g.AddGroup(botanist.MigrateAndDeleteControlPlaneTaskGroup())
		waitUntilShootManagedResourcesDeleted     = g.Add(flow.Task{
			Name:         "Waiting until shoot managed resources have been deleted",
			Fn:           flow.TaskFn(botanist.WaitUntilShootManagedResourcesDeleted).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       !cleanupShootResources,
			Dependencies: flow.NewTaskIDs(migrateControlPlane),
		})
		deleteKubeAPIServer = g.Add(flow.Task{
			Name:         "Deleting kube-apiserver deployment",
			Fn:           flow.TaskFn(botanist.DeleteKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(migrateManagedResources, waitUntilEtcdReady, migrateControlPlane, waitUntilShootManagedResourcesDeleted),
		})
		waitUntilKubeAPIServerDeleted = g.Add(flow.Task{
			Name:         "Waiting until kube-apiserver has been deleted",
			Fn:           botanist.Shoot.Components.ControlPlane.KubeAPIServer.WaitCleanup,
			Dependencies: flow.NewTaskIDs(deleteKubeAPIServer),
		})

		migrateExtensionsAfterKubeAPIServer = g.AddGroup(botanist.MigrateAndDeleteExtensionsAfterKubeAPIServerTaskGroup().WithDependencies(waitUntilKubeAPIServerDeleted))
		migrateInfrastructure               = g.AddGroup(botanist.MigrateAndDeleteInfrastructureTaskGroup().WithDependencies(waitUntilKubeAPIServerDeleted))
		migrateIngressDNSRecord             = g.Add(flow.Task{
			Name:         "Migrating nginx ingress DNS record",
			Fn:           botanist.MigrateIngressDNSRecord,
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		migrateExternalDNSRecord = g.Add(flow.Task{
			Name:         "Migrating external domain DNS record",
			Fn:           botanist.MigrateExternalDNSRecord,
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		migrateInternalDNSRecord = g.Add(flow.Task{
			Name:         "Migrating internal domain DNS record",
			Fn:           botanist.MigrateInternalDNSRecord,
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		syncPoint = flow.NewTaskIDs(
			migrateExtensionsAfterKubeAPIServer,
			destroyExtensionAndMachineResourcesGroup,
			migrateInfrastructure,
		)
		destroyDNSRecords = g.Add(flow.Task{
			Name:         "Deleting DNSRecords from the Shoot namespace",
			Fn:           botanist.DestroyDNSRecords,
			SkipIf:       !nonTerminatingNamespace,
			Dependencies: flow.NewTaskIDs(syncPoint, migrateIngressDNSRecord, migrateExternalDNSRecord, migrateInternalDNSRecord),
		})
		createETCDSnapshot = g.Add(flow.Task{
			Name:         "Creating ETCD Snapshot",
			Fn:           botanist.SnapshotEtcd,
			SkipIf:       !etcdSnapshotRequired,
			Dependencies: flow.NewTaskIDs(syncPoint, waitUntilKubeAPIServerDeleted),
		})
		migrateBackupEntryInGarden = g.Add(flow.Task{
			Name:         "Migrating BackupEntry to new seed",
			Fn:           botanist.Shoot.Components.BackupEntry.Migrate,
			Dependencies: flow.NewTaskIDs(syncPoint, createETCDSnapshot),
		})
		waitUntilBackupEntryInGardenMigrated = g.Add(flow.Task{
			Name:         "Waiting for BackupEntry to be migrated to new seed",
			Fn:           botanist.Shoot.Components.BackupEntry.WaitMigrate,
			Dependencies: flow.NewTaskIDs(migrateBackupEntryInGarden),
		})
		destroyEtcd = g.Add(flow.Task{
			Name:         "Destroying main and events etcd",
			Fn:           flow.TaskFn(botanist.DestroyEtcd).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPoint, createETCDSnapshot, waitUntilBackupEntryInGardenMigrated),
		})
		waitUntilEtcdDeleted = g.Add(flow.Task{
			Name:         "Waiting until main and event etcd have been destroyed",
			Fn:           flow.TaskFn(botanist.WaitUntilEtcdsDeleted).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(destroyEtcd),
		})
		deleteNamespace = g.Add(flow.Task{
			Name:         "Deleting shoot namespace in Seed",
			Fn:           flow.TaskFn(botanist.DeleteSeedNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(syncPoint, waitUntilBackupEntryInGardenMigrated, destroyDNSRecords, migrateManagedResources, waitUntilEtcdDeleted),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until shoot namespace in Seed has been deleted",
			Fn:           botanist.WaitUntilSeedNamespaceDeleted,
			Dependencies: flow.NewTaskIDs(deleteNamespace),
		})

		f = g.Compile()
	)

	if err := f.Run(ctx, flow.Opts{
		Log:              o.Logger,
		ProgressReporter: r.newProgressReporter(o.ReportShootProgress),
		ErrorContext:     errorContext,
		ErrorCleaner:     o.CleanShootTaskError,
	}); err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), flow.Errors(err))
	}

	o.Logger.Info("Successfully prepared Shoot cluster for restoration")
	return nil
}
