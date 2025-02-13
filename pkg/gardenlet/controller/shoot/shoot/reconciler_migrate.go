// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
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

	nodeAgentAuthorizerWebhookReady, err := botanist.IsGardenerResourceManagerReady(ctx)
	if err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
	}

	var (
		g = flow.NewGraph("Shoot cluster preparation for migration")

		deployNamespace = g.Add(flow.Task{
			Name:   "Deploying Shoot namespace in Seed",
			Fn:     flow.TaskFn(botanist.DeploySeedNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf: !nonTerminatingNamespace,
		})
		initializeSecretsManagement = g.Add(flow.Task{
			Name:         "Initializing secrets management",
			Fn:           flow.TaskFn(botanist.InitializeSecretsManagement).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       !nonTerminatingNamespace,
			Dependencies: flow.NewTaskIDs(deployNamespace),
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
			Name: "Scaling Kubernetes API Server up and waiting until ready",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.WakeUpKubeAPIServer(ctx, nodeAgentAuthorizerWebhookReady && features.DefaultFeatureGate.Enabled(features.NodeAgentAuthorizer))
			}),
			SkipIf:       !wakeupRequired,
			Dependencies: flow.NewTaskIDs(deployETCD, scaleUpETCD, initializeSecretsManagement),
		})
		// Deploy gardener-resource-manager to re-run the bootstrap logic if needed (e.g. when the token is expired because of hibernation).
		// This fixes https://github.com/gardener/gardener/issues/7606
		deployGardenerResourceManager = g.Add(flow.Task{
			Name:         "Deploying gardener-resource-manager",
			Fn:           botanist.DeployGardenerResourceManager,
			SkipIf:       !cleanupShootResources,
			Dependencies: flow.NewTaskIDs(wakeUpKubeAPIServer),
		})
		ensureResourceManagerScaledUp = g.Add(flow.Task{
			Name:         "Ensuring that the gardener-resource-manager is scaled to 1",
			Fn:           botanist.ScaleGardenerResourceManagerToOne,
			SkipIf:       !cleanupShootResources,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		// TODO(oliver-goetz): Consider removing this two-step deployment once we only support Kubernetes 1.32+ (in this
		//  version, the structured authorization feature has been promoted to GA). We already use structured authz for
		//  1.30+ clusters. This is similar to kube-apiserver deployment in gardener-operator.
		//  See https://github.com/gardener/gardener/pull/10682#discussion_r1816324389 for more information.
		wakeUpKubeAPIServerWithNodeAgentAuthorizer = g.Add(flow.Task{
			Name: "Scaling Kubernetes API Server with node-agent-authorizer up and waiting until ready",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.WakeUpKubeAPIServer(ctx, true)
			}),
			SkipIf:       !wakeupRequired || nodeAgentAuthorizerWebhookReady || !features.DefaultFeatureGate.Enabled(features.NodeAgentAuthorizer),
			Dependencies: flow.NewTaskIDs(ensureResourceManagerScaledUp),
		})
		keepManagedResourcesObjectsInShoot = g.Add(flow.Task{
			Name:         "Configuring Managed Resources objects to be kept in the Shoot",
			Fn:           botanist.KeepObjectsForManagedResources,
			SkipIf:       !cleanupShootResources,
			Dependencies: flow.NewTaskIDs(ensureResourceManagerScaledUp, wakeUpKubeAPIServerWithNodeAgentAuthorizer),
		})
		deleteManagedResources = g.Add(flow.Task{
			Name:         "Deleting all Managed Resources from the Shoot's namespace",
			Fn:           botanist.DeleteManagedResources,
			Dependencies: flow.NewTaskIDs(keepManagedResourcesObjectsInShoot, ensureResourceManagerScaledUp, wakeUpKubeAPIServerWithNodeAgentAuthorizer),
		})
		waitForManagedResourcesDeletion = g.Add(flow.Task{
			Name:         "Waiting until ManagedResources are deleted",
			Fn:           flow.TaskFn(botanist.WaitUntilManagedResourcesDeleted).Timeout(10 * time.Minute),
			Dependencies: flow.NewTaskIDs(deleteManagedResources),
		})
		deleteMachineControllerManager = g.Add(flow.Task{
			Name: "Deleting machine-controller-manager",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.ControlPlane.MachineControllerManager.Destroy(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(waitForManagedResourcesDeletion),
		})
		waitUntilMachineControllerManagerDeleted = g.Add(flow.Task{
			Name: "Waiting until machine-controller-manager has been deleted",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.ControlPlane.MachineControllerManager.WaitCleanup(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deleteMachineControllerManager),
		})
		migrateExtensionResources = g.Add(flow.Task{
			Name:         "Migrating extension resources",
			Fn:           botanist.MigrateExtensionResourcesInParallel,
			Dependencies: flow.NewTaskIDs(waitUntilMachineControllerManagerDeleted),
		})
		waitUntilExtensionResourcesMigrated = g.Add(flow.Task{
			Name:         "Waiting until extension resources have been migrated",
			Fn:           botanist.WaitUntilExtensionResourcesMigrated,
			Dependencies: flow.NewTaskIDs(migrateExtensionResources),
		})
		migrateExtensionsBeforeKubeAPIServer = g.Add(flow.Task{
			Name:         "Migrating extensions before kube-apiserver",
			Fn:           botanist.Shoot.Components.Extensions.Extension.MigrateBeforeKubeAPIServer,
			Dependencies: flow.NewTaskIDs(waitForManagedResourcesDeletion),
		})
		waitUntilExtensionsBeforeKubeAPIServerMigrated = g.Add(flow.Task{
			Name:         "Waiting until extensions that should be handled before kube-apiserver have been migrated",
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitMigrateBeforeKubeAPIServer,
			Dependencies: flow.NewTaskIDs(migrateExtensionsBeforeKubeAPIServer),
		})
		persistShootState = g.Add(flow.Task{
			Name: "Persisting ShootState in garden cluster",
			Fn: func(ctx context.Context) error {
				return shootstate.Deploy(ctx, r.Clock, botanist.GardenClient, botanist.SeedClientSet.Client(), botanist.Shoot.GetInfo(), false)
			},
			Dependencies: flow.NewTaskIDs(waitUntilExtensionResourcesMigrated),
		})
		deleteExtensionResources = g.Add(flow.Task{
			Name:         "Deleting extension resources from the Shoot namespace",
			Fn:           botanist.DestroyExtensionResourcesInParallel,
			Dependencies: flow.NewTaskIDs(persistShootState),
		})
		waitUntilExtensionResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension resources have been deleted",
			Fn:           botanist.WaitUntilExtensionResourcesDeleted,
			Dependencies: flow.NewTaskIDs(deleteExtensionResources),
		})
		deleteMachineResources = g.Add(flow.Task{
			Name:         "Shallow-deleting machine resources from the Shoot namespace",
			Fn:           botanist.ShallowDeleteMachineResources,
			Dependencies: flow.NewTaskIDs(persistShootState),
		})
		waitUntilMachineResourcesDeleted = g.Add(flow.Task{
			Name: "Waiting until machine resources have been deleted",
			Fn: func(ctx context.Context) error {
				return gardenerutils.WaitUntilMachineResourcesDeleted(ctx, botanist.Logger, botanist.SeedClientSet.Client(), botanist.Shoot.ControlPlaneNamespace)
			},
			Dependencies: flow.NewTaskIDs(deleteMachineResources),
		})
		deleteExtensionsBeforeKubeAPIServer = g.Add(flow.Task{
			Name:         "Deleting extensions before kube-apiserver",
			Fn:           botanist.Shoot.Components.Extensions.Extension.DestroyBeforeKubeAPIServer,
			Dependencies: flow.NewTaskIDs(waitUntilExtensionResourcesDeleted, waitUntilExtensionsBeforeKubeAPIServerMigrated),
		})
		waitUntilExtensionsBeforeKubeAPIServerDeleted = g.Add(flow.Task{
			Name:         "Waiting until extensions that should be handled before kube-apiserver have been deleted",
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitCleanupBeforeKubeAPIServer,
			Dependencies: flow.NewTaskIDs(deleteExtensionsBeforeKubeAPIServer),
		})
		deleteStaleExtensionResources = g.Add(flow.Task{
			Name:         "Deleting stale extensions",
			Fn:           botanist.Shoot.Components.Extensions.Extension.DeleteStaleResources,
			Dependencies: flow.NewTaskIDs(waitUntilExtensionResourcesMigrated),
		})
		waitUntilStaleExtensionResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until all stale extensions have been deleted",
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitCleanupStaleResources,
			Dependencies: flow.NewTaskIDs(deleteStaleExtensionResources),
		})
		migrateControlPlane = g.Add(flow.Task{
			Name: "Migrating shoot control plane",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ControlPlane.Migrate(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(waitUntilExtensionResourcesDeleted, waitUntilExtensionsBeforeKubeAPIServerDeleted, waitUntilStaleExtensionResourcesDeleted),
		})
		deleteControlPlane = g.Add(flow.Task{
			Name: "Deleting shoot control plane",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ControlPlane.Destroy(ctx)
			}).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(migrateControlPlane),
		})
		waitUntilControlPlaneDeleted = g.Add(flow.Task{
			Name: "Waiting until shoot control plane has been deleted",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.ControlPlane.WaitCleanup(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deleteControlPlane),
		})
		waitUntilShootManagedResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until shoot managed resources have been deleted",
			Fn:           flow.TaskFn(botanist.WaitUntilShootManagedResourcesDeleted).RetryUntilTimeout(defaultInterval, defaultTimeout),
			SkipIf:       !cleanupShootResources,
			Dependencies: flow.NewTaskIDs(waitUntilControlPlaneDeleted),
		})
		deleteKubeAPIServer = g.Add(flow.Task{
			Name:         "Deleting kube-apiserver deployment",
			Fn:           flow.TaskFn(botanist.DeleteKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
			Dependencies: flow.NewTaskIDs(waitForManagedResourcesDeletion, waitUntilEtcdReady, waitUntilControlPlaneDeleted, waitUntilShootManagedResourcesDeleted),
		})
		waitUntilKubeAPIServerDeleted = g.Add(flow.Task{
			Name:         "Waiting until kube-apiserver has been deleted",
			Fn:           botanist.Shoot.Components.ControlPlane.KubeAPIServer.WaitCleanup,
			Dependencies: flow.NewTaskIDs(deleteKubeAPIServer),
		})
		migrateExtensionsAfterKubeAPIServer = g.Add(flow.Task{
			Name:         "Migrating extensions after kube-apiserver",
			Fn:           botanist.Shoot.Components.Extensions.Extension.MigrateAfterKubeAPIServer,
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		waitUntilExtensionsAfterKubeAPIServerMigrated = g.Add(flow.Task{
			Name:         "Waiting until extensions that should be handled after kube-apiserver have been migrated",
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitMigrateAfterKubeAPIServer,
			Dependencies: flow.NewTaskIDs(migrateExtensionsAfterKubeAPIServer),
		})
		deleteExtensionsAfterKubeAPIServer = g.Add(flow.Task{
			Name:         "Deleting extensions after kube-apiserver",
			Fn:           botanist.Shoot.Components.Extensions.Extension.DestroyAfterKubeAPIServer,
			Dependencies: flow.NewTaskIDs(waitUntilExtensionsAfterKubeAPIServerMigrated),
		})
		waitUntilExtensionsAfterKubeAPIServerDeleted = g.Add(flow.Task{
			Name:         "Waiting until extensions that should be handled after kube-apiserver have been deleted",
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitCleanupAfterKubeAPIServer,
			Dependencies: flow.NewTaskIDs(deleteExtensionsAfterKubeAPIServer),
		})
		// Add this step in interest of completeness. All extension deletions should have already been triggered by previous steps.
		waitUntilExtensionsDeleted = g.Add(flow.Task{
			Name:         "Waiting until all extensions have been deleted",
			Fn:           botanist.Shoot.Components.Extensions.Extension.WaitCleanup,
			Dependencies: flow.NewTaskIDs(waitUntilExtensionsAfterKubeAPIServerMigrated),
		})
		migrateInfrastructure = g.Add(flow.Task{
			Name: "Migrating shoot infrastructure",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.Infrastructure.Migrate(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(waitUntilKubeAPIServerDeleted),
		})
		waitUntilInfrastructureMigrated = g.Add(flow.Task{
			Name: "Waiting until shoot infrastructure has been migrated",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.Infrastructure.WaitMigrate(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(migrateInfrastructure),
		})
		deleteInfrastructure = g.Add(flow.Task{
			Name: "Deleting shoot infrastructure",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.Infrastructure.Destroy(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(waitUntilInfrastructureMigrated),
		})
		waitUntilInfrastructureDeleted = g.Add(flow.Task{
			Name: "Waiting until shoot infrastructure has been deleted",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return botanist.Shoot.Components.Extensions.Infrastructure.WaitCleanup(ctx)
			}),
			SkipIf:       o.Shoot.IsWorkerless,
			Dependencies: flow.NewTaskIDs(deleteInfrastructure),
		})
		migrateIngressDNSRecord = g.Add(flow.Task{
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
			waitUntilExtensionsAfterKubeAPIServerDeleted,
			waitUntilMachineResourcesDeleted,
			waitUntilExtensionsDeleted,
			waitUntilInfrastructureDeleted,
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
			Dependencies: flow.NewTaskIDs(syncPoint, waitUntilBackupEntryInGardenMigrated, deleteExtensionResources, destroyDNSRecords, waitForManagedResourcesDeletion, waitUntilEtcdDeleted),
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
