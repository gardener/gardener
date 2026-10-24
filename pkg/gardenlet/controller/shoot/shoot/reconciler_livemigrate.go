// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

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

const (
	liveMigrationStepInterval = 10 * time.Second
	liveMigrationStepTimeout  = 10 * time.Minute
)

// liveMigrationStepOwners maps each live control plane migration condition to the gardenlet role responsible for
// executing that step. The flow graph looks up each step's owner here to decide to perform the step or wait for the peer gardenlet.
var liveMigrationStepOwners = map[gardencorev1beta1.ConditionType]v1beta1helper.LiveMigrationRole{
	gardencorev1beta1.ShootLiveMigrationSourceEtcdPreparedForPeerJoin:              v1beta1helper.LiveMigrationRoleSource,
	gardencorev1beta1.ShootLiveMigrationDestinationEtcdPeersJoined:                 v1beta1helper.LiveMigrationRoleDestination,
	gardencorev1beta1.ShootLiveMigrationMigrateExtensionsNeededBeforeKubeAPIServer: v1beta1helper.LiveMigrationRoleSource,
	gardencorev1beta1.ShootLiveMigrationDestinationKubeAPIServerReady:              v1beta1helper.LiveMigrationRoleDestination,
	gardencorev1beta1.ShootLiveMigrationMigrateDNSRecords:                          v1beta1helper.LiveMigrationRoleSource,
	gardencorev1beta1.ShootLiveMigrationEtcdMigrationComplete:                      v1beta1helper.LiveMigrationRoleDestination,
	gardencorev1beta1.ShootLiveMigrationSourceSeedCleanup:                          v1beta1helper.LiveMigrationRoleSource,
	gardencorev1beta1.ShootLiveMigrationMigrationCompleted:                         v1beta1helper.LiveMigrationRoleDestination,
	gardencorev1beta1.ShootLiveMigrationSourceSeedFinalTeardown:                    v1beta1helper.LiveMigrationRoleSource,
}

// liveMigrationConditionOrder is the canonical ordering of live migration conditions, matching the flow step sequence.
// Conditions are sorted by this order when written to status so they always appear in flow order regardless of which
// gardenlet set them first.
var liveMigrationConditionOrder = []gardencorev1beta1.ConditionType{
	gardencorev1beta1.ShootLiveMigrationSourceEtcdPreparedForPeerJoin,
	gardencorev1beta1.ShootLiveMigrationDestinationEtcdPeersJoined,
	gardencorev1beta1.ShootLiveMigrationMigrateExtensionsNeededBeforeKubeAPIServer,
	gardencorev1beta1.ShootLiveMigrationDestinationKubeAPIServerReady,
	gardencorev1beta1.ShootLiveMigrationMigrateDNSRecords,
	gardencorev1beta1.ShootLiveMigrationEtcdMigrationComplete,
	gardencorev1beta1.ShootLiveMigrationSourceSeedCleanup,
	gardencorev1beta1.ShootLiveMigrationMigrationCompleted,
	gardencorev1beta1.ShootLiveMigrationSourceSeedFinalTeardown,
}

func (r *Reconciler) runLiveMigrateShootFlow(ctx context.Context, o *operation.Operation, role v1beta1helper.LiveMigrationRole) *v1beta1helper.WrappedLastErrors {
	var (
		botanist        *botanistpkg.Botanist
		err             error
		tasksWithErrors []string
	)

	for _, lastError := range o.Shoot.GetInfo().Status.LastErrors {
		if lastError.TaskID != nil {
			tasksWithErrors = append(tasksWithErrors, *lastError.TaskID)
		}
	}

	errorContext := errorsutils.NewErrorContext("Shoot control plane live migration", tasksWithErrors)

	if err = errorsutils.HandleErrors(errorContext,
		func(errorID string) error {
			o.CleanShootTaskError(ctx, errorID)
			return nil
		},
		nil,
		errorsutils.ToExecute("Create botanist", func() error {
			return retryutils.UntilTimeout(ctx, liveMigrationStepInterval, liveMigrationStepTimeout, func(context.Context) (done bool, err error) {
				botanist, err = botanistpkg.New(ctx, o)
				if err != nil {
					return retryutils.MinorError(err)
				}
				return retryutils.Ok()
			})
		}),
	); err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
	}

	// Set before the graph runs so IsRestorePhase() is consistent across every task.
	o.Shoot.LiveMigrationRole = role

	// The live migration flow deploys the shoot control plane on the destination seed without running the
	// infrastructure reconciliation that normally populates o.Shoot.Networks (see WaitForInfrastructure). Compute
	// the networks here from the shoot's spec and status so control plane components (e.g. kube-apiserver, VPN) can
	// read the CIDRs. This mirrors the regular reconcile and migrate flows.
	if o.Shoot.GetInfo().Spec.Networking != nil && o.Shoot.GetInfo().Spec.Networking.Nodes != nil {
		networks, err := shoot.ToNetworks(o.Shoot.GetInfo(), o.Shoot.IsWorkerless)
		if err != nil {
			return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), err)
		}
		o.Shoot.Networks = networks
	}

	var (
		g = flow.NewGraph("Shoot control plane live migration")

		sourceEtcdReadyForPeerJoin = g.Add(flow.Task{
			Name: "Making source etcd ready for peer join",
			Fn:   r.executeStepOrWait(botanist, role, gardencorev1beta1.ShootLiveMigrationSourceEtcdPreparedForPeerJoin, sourceEtcdPreparedForPeerJoinTaskGroup(botanist)),
		})

		destinationEtcdJoined = g.Add(flow.Task{
			Name:         "Joining destination etcd to the source cluster",
			Fn:           r.executeStepOrWait(botanist, role, gardencorev1beta1.ShootLiveMigrationDestinationEtcdPeersJoined, destinationEtcdPeersJoinedTaskGroup(botanist)),
			Dependencies: flow.NewTaskIDs(sourceEtcdReadyForPeerJoin),
		})

		migrateExtensionsBeforeKubeAPIServer = g.Add(flow.Task{
			Name:         "Migrating source extensions needed before kube-apiserver",
			Fn:           r.executeStepOrWait(botanist, role, gardencorev1beta1.ShootLiveMigrationMigrateExtensionsNeededBeforeKubeAPIServer, botanist.MigrateExtensionsBeforeKubeAPIServerTaskGroup()),
			Dependencies: flow.NewTaskIDs(destinationEtcdJoined),
		})

		destinationKubeAPIServerReady = g.Add(flow.Task{
			Name:         "Deploying destination control plane and temporary VPN",
			Fn:           r.executeStepOrWait(botanist, role, gardencorev1beta1.ShootLiveMigrationDestinationKubeAPIServerReady, destinationKubeAPIServerReadyTaskGroup(botanist)),
			Dependencies: flow.NewTaskIDs(migrateExtensionsBeforeKubeAPIServer),
		})

		dnsRecordsMigrated = g.Add(flow.Task{
			Name:         "Migrating DNS records to the destination seed",
			Fn:           r.executeStepOrWait(botanist, role, gardencorev1beta1.ShootLiveMigrationMigrateDNSRecords, migrateDNSRecordsTaskGroup(botanist)),
			Dependencies: flow.NewTaskIDs(destinationKubeAPIServerReady),
		})

		etcdMigrationComplete = g.Add(flow.Task{
			Name:         "Removing source etcd members from the joined cluster",
			Fn:           r.executeStepOrWait(botanist, role, gardencorev1beta1.ShootLiveMigrationEtcdMigrationComplete, etcdMigrationCompleteTaskGroup()),
			Dependencies: flow.NewTaskIDs(dnsRecordsMigrated),
		})

		sourceSeedCleanup = g.Add(flow.Task{
			Name:         "Cleaning up the source seed",
			Fn:           r.executeStepOrWait(botanist, role, gardencorev1beta1.ShootLiveMigrationSourceSeedCleanup, sourceSeedCleanupTaskGroup(botanist)),
			Dependencies: flow.NewTaskIDs(etcdMigrationComplete),
		})

		migrationCompleted = g.Add(flow.Task{
			Name:         "Completing live migration",
			Fn:           r.executeStepOrWait(botanist, role, gardencorev1beta1.ShootLiveMigrationMigrationCompleted, destinationCompletionTaskGroup(botanist)),
			Dependencies: flow.NewTaskIDs(sourceSeedCleanup),
		})

		_ = g.Add(flow.Task{
			Name:         "Cleaning up the source VPN server and namespace",
			Fn:           r.executeStepOrWait(botanist, role, gardencorev1beta1.ShootLiveMigrationSourceSeedFinalTeardown, sourceSeedFinalTeardownTaskGroup(botanist)),
			Dependencies: flow.NewTaskIDs(migrationCompleted),
		})
	)

	f := g.Compile()
	if err := f.Run(ctx, flow.Opts{
		Log:              botanist.Logger,
		ProgressReporter: r.newProgressReporter(botanist.ReportShootProgress),
		ErrorContext:     errorContext,
		ErrorCleaner:     botanist.CleanShootTaskError,
	}); err != nil {
		return v1beta1helper.NewWrappedLastErrors(v1beta1helper.FormatLastErrDescription(err), flow.Errors(err))
	}

	return nil
}

// executeStepOrWait returns a TaskFn that either executes the tasks in the given group sequentially (if this
// gardenlet owns the step) or waits for the peer gardenlet to complete it.
func (r *Reconciler) executeStepOrWait(botanist *botanistpkg.Botanist, role v1beta1helper.LiveMigrationRole, conditionType gardencorev1beta1.ConditionType, group flow.TaskGroup) flow.TaskFn {
	owner := liveMigrationStepOwners[conditionType]
	return flow.TaskFn(func(ctx context.Context) error {
		if role != owner {
			return r.waitForLiveMigrationPeerStep(ctx, botanist, conditionType)
		}
		if err := r.setLiveMigrationStepCondition(ctx, botanist.Shoot.GetInfo(), conditionType, false); err != nil {
			return err
		}

		g := flow.NewGraph(string(conditionType))
		g.AddGroup(group)
		if err := g.Compile().Run(ctx, flow.Opts{Log: botanist.Logger}); err != nil {
			taskErr := flow.Errors(err)
			if condErr := r.setLiveMigrationStepConditionError(ctx, botanist.Shoot.GetInfo(), conditionType, taskErr); condErr != nil {
				botanist.Logger.Error(condErr, "Failed to set error condition for live migration step", "step", conditionType)
			}
			return taskErr
		}
		return r.setLiveMigrationStepCondition(ctx, botanist.Shoot.GetInfo(), conditionType, true)
	}).RetryUntilTimeout(liveMigrationStepInterval, liveMigrationStepTimeout)
}

func (r *Reconciler) waitForLiveMigrationPeerStep(ctx context.Context, botanist *botanistpkg.Botanist, conditionType gardencorev1beta1.ConditionType) error {
	shoot := &gardencorev1beta1.Shoot{}
	if err := r.GardenClient.Get(ctx, client.ObjectKeyFromObject(botanist.Shoot.GetInfo()), shoot); err != nil {
		return fmt.Errorf("failed to read shoot while waiting for peer gardenlet to complete live migration step %q: %w", conditionType, err)
	}

	if !v1beta1helper.IsLiveMigrationConditionTrue(shoot, conditionType) {
		botanist.Logger.Info("Waiting for peer gardenlet to complete live migration step", "step", conditionType)
		return fmt.Errorf("waiting for peer gardenlet to complete live migration step %q", conditionType)
	}

	return nil
}

func (r *Reconciler) setLiveMigrationStepCondition(ctx context.Context, shoot *gardencorev1beta1.Shoot, conditionType gardencorev1beta1.ConditionType, done bool) error {
	condition := v1beta1helper.GetOrInitConditionWithClock(r.Clock, v1beta1helper.GetLiveMigrationConditions(shoot), conditionType)
	if done {
		condition = v1beta1helper.UpdatedConditionWithClock(r.Clock, condition, gardencorev1beta1.ConditionTrue, "StepCompleted", "The live migration step has been completed.")
	} else {
		condition = v1beta1helper.UpdatedConditionWithClock(r.Clock, condition, gardencorev1beta1.ConditionProgressing, "StepInProgress", "The live migration step is in progress.")
	}
	return r.patchLiveMigrationConditions(ctx, shoot, condition)
}

func (r *Reconciler) setLiveMigrationStepConditionError(ctx context.Context, shoot *gardencorev1beta1.Shoot, conditionType gardencorev1beta1.ConditionType, err error) error {
	condition := v1beta1helper.GetOrInitConditionWithClock(r.Clock, v1beta1helper.GetLiveMigrationConditions(shoot), conditionType)
	condition = v1beta1helper.UpdatedConditionWithClock(r.Clock, condition, gardencorev1beta1.ConditionFalse, "StepFailed", err.Error())
	return r.patchLiveMigrationConditions(ctx, shoot, condition)
}

// finalizeLiveMigration switches the shoot's status.seedName to the destination seed and clears the live-migration
// state and intent annotation, returning the shoot to normal reconciliation on the destination seed.
func (r *Reconciler) finalizeLiveMigration(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	patch := client.MergeFrom(shoot.DeepCopy())
	delete(shoot.Annotations, v1beta1constants.AnnotationMigrationLiveMigrate)
	if err := r.GardenClient.Patch(ctx, shoot, patch); err != nil {
		return fmt.Errorf("failed to remove live migration annotation: %w", err)
	}

	statusPatch := client.StrategicMergeFrom(shoot.DeepCopy())
	shoot.Status.SeedName = shoot.Spec.SeedName
	shoot.Status.LiveMigration = nil
	shoot.Status.MigrationStartTime = nil
	return r.GardenClient.Status().Patch(ctx, shoot, statusPatch)
}

func (r *Reconciler) patchLiveMigrationConditions(ctx context.Context, shoot *gardencorev1beta1.Shoot, conditions ...gardencorev1beta1.Condition) error {
	patch := client.StrategicMergeFrom(shoot.DeepCopy())

	if shoot.Status.LiveMigration == nil {
		shoot.Status.LiveMigration = &gardencorev1beta1.LiveMigration{}
	}
	merged := v1beta1helper.MergeConditions(shoot.Status.LiveMigration.Conditions, conditions...)
	slices.SortStableFunc(merged, func(a, b gardencorev1beta1.Condition) int {
		return cmp.Compare(slices.Index(liveMigrationConditionOrder, a.Type), slices.Index(liveMigrationConditionOrder, b.Type))
	})
	shoot.Status.LiveMigration.Conditions = merged

	return r.GardenClient.Status().Patch(ctx, shoot, patch)
}

// taskGroupSourceEtcdPreparedForPeerJoin is a flow.TaskID for a logical flow.TaskGroup.
const taskGroupSourceEtcdPreparedForPeerJoin flow.TaskID = "TaskGroupSourceEtcdPreparedForPeerJoin"

// sourceEtcdPreparedForPeerJoinTaskGroup returns the flow.TaskGroup for making the source etcd ready for peer join.
func sourceEtcdPreparedForPeerJoinTaskGroup(b *botanistpkg.Botanist) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(taskGroupSourceEtcdPreparedForPeerJoin)

		deployEtcdPeerExposure = g.Add(flow.Task{
			Name: "Deploying etcd member exposure",
			Fn:   b.DeployEtcdMemberExposure,
		})
		persistShootState = g.Add(flow.Task{
			Name: "Persisting shoot state",
			Fn: func(ctx context.Context) error {
				return b.PersistShootState(ctx)
			},
			Dependencies: flow.NewTaskIDs(deployEtcdPeerExposure),
		})
		initSecretsManagement = g.Add(flow.Task{
			Name:         "Initializing secrets management",
			Fn:           b.InitializeSecretsManagement,
			Dependencies: flow.NewTaskIDs(persistShootState),
		})
		deployEtcd = g.Add(flow.Task{
			Name:         "Deploying etcd",
			Fn:           b.DeployEtcd,
			Dependencies: flow.NewTaskIDs(initSecretsManagement),
		})
		waitEtcdsReady = g.Add(flow.Task{
			Name:         "Waiting until etcds are ready",
			Fn:           b.WaitUntilEtcdsReady,
			Dependencies: flow.NewTaskIDs(deployEtcd),
		})

		// The destination restores the backup entry in DestinationEtcdPeersJoinedTaskGroup. Skip
		// re-migrating once destination has already started consuming the result.
		skipIfDestJoined = v1beta1helper.GetLiveMigrationCondition(b.Shoot.GetInfo(), gardencorev1beta1.ShootLiveMigrationDestinationEtcdPeersJoined) != nil

		migrateBackupEntry = g.Add(flow.Task{
			Name:         "Migrating backup entry",
			Fn:           b.Shoot.Components.BackupEntry.Migrate,
			SkipIf:       skipIfDestJoined,
			Dependencies: flow.NewTaskIDs(waitEtcdsReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until backup entry has been migrated",
			Fn:           b.Shoot.Components.BackupEntry.WaitMigrate,
			SkipIf:       skipIfDestJoined,
			Dependencies: flow.NewTaskIDs(migrateBackupEntry),
		})
	)

	return g
}

// taskGroupDestinationEtcdPeersJoined is a flow.TaskID for a logical flow.TaskGroup.
const taskGroupDestinationEtcdPeersJoined flow.TaskID = "TaskGroupDestinationEtcdPeersJoined"

// destinationEtcdPeersJoinedTaskGroup returns the flow.TaskGroup for joining the destination etcd to the source cluster.
func destinationEtcdPeersJoinedTaskGroup(b *botanistpkg.Botanist) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(taskGroupDestinationEtcdPeersJoined)

		// Fetch the ShootState written by the source so InitializeSecretsManagement can restore from it.
		refreshShootState = g.Add(flow.Task{
			Name: "Refreshing shoot state",
			Fn:   b.RefreshShootStateForLiveMigration,
		})
		deployControlPlaneNamespace = g.Add(flow.Task{
			Name: "Deploying control plane namespace",
			Fn:   b.DeployControlPlaneNamespace,
		})
		deployEtcdPeerExposure = g.Add(flow.Task{
			Name:         "Deploying etcd member exposure",
			Fn:           b.DeployEtcdMemberExposure,
			Dependencies: flow.NewTaskIDs(deployControlPlaneNamespace),
		})
		initSecretsManagement = g.Add(flow.Task{
			Name:         "Initializing secrets management",
			Fn:           b.InitializeSecretsManagement,
			Dependencies: flow.NewTaskIDs(refreshShootState, deployEtcdPeerExposure),
		})
		// The source backup entry is deployed to ensure that the data in the source seed's backup bucket
		// is properly cleaned up at a later stage of the flow.
		deploySourceBackupEntry = g.Add(flow.Task{
			Name:         "Deploying source backup entry",
			Fn:           b.DeploySourceBackupEntry,
			Dependencies: flow.NewTaskIDs(deployControlPlaneNamespace),
		})
		waitSourceBackupEntryReady = g.Add(flow.Task{
			Name:         "Waiting until source backup entry is ready",
			Fn:           b.Shoot.Components.SourceBackupEntry.Wait,
			Dependencies: flow.NewTaskIDs(deploySourceBackupEntry),
		})
		restoreBackupEntry = g.Add(flow.Task{
			Name: "Restoring backup entry",
			Fn: func(ctx context.Context) error {
				return b.Shoot.Components.BackupEntry.Restore(ctx, nil)
			},
			Dependencies: flow.NewTaskIDs(waitSourceBackupEntryReady),
		})
		waitBackupEntryReady = g.Add(flow.Task{
			Name:         "Waiting until backup entry is ready",
			Fn:           b.Shoot.Components.BackupEntry.Wait,
			Dependencies: flow.NewTaskIDs(restoreBackupEntry),
		})
		deployEtcd = g.Add(flow.Task{
			Name:         "Deploying etcd",
			Fn:           b.DeployEtcd,
			Dependencies: flow.NewTaskIDs(initSecretsManagement, deployEtcdPeerExposure, waitBackupEntryReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until etcds are ready",
			Fn:           b.WaitUntilEtcdsReady,
			Dependencies: flow.NewTaskIDs(deployEtcd),
		})
	)

	return g
}

// taskGroupDestinationKubeAPIServerReady is a flow.TaskID for a logical flow.TaskGroup.
const taskGroupDestinationKubeAPIServerReady flow.TaskID = "TaskGroupDestinationKubeAPIServerReady"

// destinationKubeAPIServerReadyTaskGroup brings up the destination control plane and the temporary VPN.
func destinationKubeAPIServerReadyTaskGroup(b *botanistpkg.Botanist) flow.TaskGroup {
	g := flow.NewTaskGroup(taskGroupDestinationKubeAPIServerReady)

	refreshShootState := g.Add(flow.Task{
		Name: "Refreshing shoot state",
		Fn:   b.RefreshShootStateForLiveMigration,
	})
	deployVPNServer := g.Add(flow.Task{
		Name:         "Deploying destination VPN server",
		Fn:           b.DeployVPNServer,
		Dependencies: flow.NewTaskIDs(refreshShootState),
	})
	// The istio ConfigMap gates the pod-kube-apiserver-load-balancing webhook; without it control plane pods resolve
	// api.<internalDomain> to the source apiserver and fail authentication (401).
	reconcileIstioConfigMap := g.Add(flow.Task{
		Name:         "Reconciling istio internal load balancing configmap",
		Fn:           b.ReconcileIstioInternalLoadBalancingConfigMap,
		Dependencies: flow.NewTaskIDs(refreshShootState),
	})

	reconcileExtensions := b.ReconcileExtensionsBeforeKubeAPIServerTaskGroup(false)
	g = g.AddAll(reconcileExtensions.WithTaskDependencies(reconcileIstioConfigMap)...)

	deployService := g.Add(flow.Task{
		Name:         "Deploying kube-apiserver service",
		Fn:           b.Shoot.Components.ControlPlane.KubeAPIServerService.Deploy,
		Dependencies: flow.NewTaskIDs(reconcileExtensions),
	})
	waitService := g.Add(flow.Task{
		Name:         "Waiting until kube-apiserver service is ready",
		Fn:           b.Shoot.Components.ControlPlane.KubeAPIServerService.Wait,
		Dependencies: flow.NewTaskIDs(deployService),
	})

	reconcileKubeAPIServer := b.ReconcileKubeAPIServerTaskGroup(false, 30*time.Second)
	g = g.AddAll(reconcileKubeAPIServer.WithTaskDependencies(waitService, deployVPNServer)...)

	reconcileGardenerResourceManager := b.ReconcileGardenerResourceManagerTaskGroup(true, false)
	g = g.AddAll(reconcileGardenerResourceManager.WithTaskDependencies(reconcileKubeAPIServer)...)

	reconcileControlPlaneComponents := b.ReconcileControlPlaneComponentsTaskGroup(false)
	g = g.AddAll(reconcileControlPlaneComponents.WithTaskDependencies(reconcileGardenerResourceManager)...)

	// TODO(GEP-39): Deploy temporary VPN shoot client and live migration VPN DNS record as part of the live migration process.

	deployGardenerAccess := g.Add(flow.Task{
		Name:         "Deploying Gardener shoot access resources",
		Fn:           flow.TaskFn(b.Shoot.Components.GardenerAccess.Deploy).RetryUntilTimeout(5*time.Second, 30*time.Second),
		Dependencies: flow.NewTaskIDs(reconcileGardenerResourceManager),
	})
	// WaitUntilTemporaryTunnelConnectionExists lists pods through b.ShootClientSet, which is nil until the shoot client
	// is initialized against the freshly deployed destination kube-apiserver.
	_ = g.Add(flow.Task{
		Name:         "Initializing connection to shoot",
		Fn:           flow.TaskFn(b.InitializeDesiredShootClients).RetryUntilTimeout(5*time.Second, 2*time.Minute),
		Dependencies: flow.NewTaskIDs(deployGardenerAccess),
	})

	// TODO(GEP-39): Wait until the temporary VPN tunnel connection to the destination is established.

	return g
}

// taskGroupMigrateDNSRecords is a flow.TaskID for a logical flow.TaskGroup.
const taskGroupMigrateDNSRecords flow.TaskID = "TaskGroupMigrateDNSRecords"

// migrateDNSRecordsTaskGroup releases the source internal and external DNSRecords so the destination can take them over.
func migrateDNSRecordsTaskGroup(b *botanistpkg.Botanist) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(taskGroupMigrateDNSRecords).
			SkipIf(v1beta1helper.GetLiveMigrationCondition(b.Shoot.GetInfo(), gardencorev1beta1.ShootLiveMigrationSourceSeedCleanup) != nil)

		_ = g.Add(flow.Task{
			Name: "Migrating internal DNS record",
			Fn:   b.MigrateInternalDNSRecord,
		})
		_ = g.Add(flow.Task{
			Name: "Migrating external DNS record",
			Fn:   b.MigrateExternalDNSRecord,
		})
	)

	return g
}

// taskGroupEtcdMigrationComplete is a flow.TaskID for a logical flow.TaskGroup.
const taskGroupEtcdMigrationComplete flow.TaskID = "TaskGroupEtcdMigrationComplete"

// etcdMigrationCompleteTaskGroup removes the source cluster's etcd members from the joined cluster, scaling etcd-main
// back to its final member count, and waits until the destination etcd is ready again.
func etcdMigrationCompleteTaskGroup() flow.TaskGroup {
	return flow.NewTaskGroup(taskGroupEtcdMigrationComplete)
}

// taskGroupSourceSeedCleanup is a flow.TaskID for a logical flow.TaskGroup.
const taskGroupSourceSeedCleanup flow.TaskID = "TaskGroupSourceSeedCleanup"

// sourceSeedCleanupTaskGroup migrates and tears down the source-side shoot resources (extensions, infrastructure,
// control plane, managed resources, machines) and persists the migrated state for the destination to restore.
func sourceSeedCleanupTaskGroup(b *botanistpkg.Botanist) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(taskGroupSourceSeedCleanup)
	)

	persistShootState := g.Add(flow.Task{
		Name: "Persisting shoot state",
		Fn:   b.PersistShootState,
	})

	migrateManagedResources := b.MigrateAndDeleteManagedResourcesTaskGroup(true)
	g = g.AddAll(migrateManagedResources.WithTaskDependencies(persistShootState)...)

	migrateExtensions := b.MigrateExtensionResourcesTaskGroup()
	g = g.AddAll(migrateExtensions.WithTaskDependencies(migrateManagedResources)...)

	destroyExtensionsAndMachineResources := b.DestroyExtensionAndMachineResourcesTaskGroup()
	g = g.AddAll(destroyExtensionsAndMachineResources.WithTaskDependencies(migrateExtensions)...)

	migrateControlPlane := b.MigrateAndDeleteControlPlaneTaskGroup()
	g = g.AddAll(migrateControlPlane.WithTaskDependencies(destroyExtensionsAndMachineResources)...)

	migrateExtensionsAfterKubeAPIServerGroup := b.MigrateAndDeleteExtensionsAfterKubeAPIServerTaskGroup()
	g = g.AddAll(migrateExtensionsAfterKubeAPIServerGroup.WithTaskDependencies(migrateControlPlane)...)

	migrateInfrastructureGroup := b.MigrateAndDeleteInfrastructureTaskGroup()
	g = g.AddAll(migrateInfrastructureGroup.WithTaskDependencies(migrateExtensionsAfterKubeAPIServerGroup)...)

	_ = g.Add(flow.Task{
		Name:         "Deleting DNSRecords from the source namespace",
		Fn:           b.DestroyDNSRecords,
		Dependencies: flow.NewTaskIDs(migrateInfrastructureGroup),
	})
	_ = g.Add(flow.Task{
		Name:         "Deleting source backup entry from the source namespace",
		Fn:           b.DestroySourceBackupEntry,
		Dependencies: flow.NewTaskIDs(migrateInfrastructureGroup),
	})

	return g
}

// taskGroupDestinationCompletionTaskGroup is a flow.TaskID for a logical flow.TaskGroup.
const taskGroupDestinationCompletionTaskGroup flow.TaskID = "TaskGroupDestinationCompletionTaskGroup"

// destinationCompletionTaskGroup returns the group run by the MigrationCompleted step: the restore chain followed by
// the destination shoot-system layer (system components, resources, addons, observability), run sequentially.
func destinationCompletionTaskGroup(b *botanistpkg.Botanist) flow.TaskGroup {
	var (
		kubeProxyEnabled = v1beta1helper.KubeProxyEnabled(b.Shoot.GetInfo().Spec.Kubernetes.KubeProxy)
		nginxDisabled    = b.Shoot.IsWorkerless || !v1beta1helper.NginxIngressEnabled(b.Shoot.GetInfo().Spec.Addons)
		g                = flow.NewTaskGroup(taskGroupDestinationCompletionTaskGroup)
	)

	refreshShootState := g.Add(flow.Task{
		Name: "Refreshing shoot state",
		Fn:   b.RefreshShootStateForLiveMigration,
	})

	deployReferencedResources := b.ReconcileReferencedResourcesTaskGroup()
	g = g.AddAll(deployReferencedResources.WithTaskDependencies(refreshShootState)...)
	// Deploy the DNSRecords first so api.<domain> resolves to the destination before the long tail of restores (and
	// before the permanent vpn-shoot, whose endpoint is api.<internalDomain>, is brought up).
	deployDNS := b.ReconcileDNSRecordsTaskGroup()
	g = g.AddAll(deployDNS.WithTaskDependencies(refreshShootState)...)

	// Give upstream DNS caches time to pick up the destination targets before bringing up the permanent vpn-shoot
	// (whose endpoint resolves via api.<internalDomain>) and the other components that resolve these names.
	waitDNSCacheSync := g.Add(flow.Task{
		Name:         "Waiting for DNS caches to sync to the destination",
		Fn:           b.WaitForLiveMigrationDNSCacheSync,
		Dependencies: flow.NewTaskIDs(deployDNS),
	})

	deployCloudProviderSecret := b.DeployCloudProviderSecretTaskGroup()
	g = g.AddAll(deployCloudProviderSecret.WithTaskDependencies(waitDNSCacheSync)...)

	reconcileInfrastructure := b.ReconcileInfrastructureTaskGroup(false)
	g = g.AddAll(reconcileInfrastructure.WithTaskDependencies(deployCloudProviderSecret)...)

	reconcileControlPlane := b.ReconcileControlPlaneTaskGroup(false)
	g = g.AddAll(reconcileControlPlane.WithTaskDependencies(reconcileInfrastructure)...)

	restoreNetwork := g.Add(flow.Task{
		Name:         "Restoring network",
		Fn:           b.DeployNetwork,
		SkipIf:       b.Shoot.IsWorkerless,
		Dependencies: flow.NewTaskIDs(reconcileControlPlane),
	})
	waitNetwork := g.Add(flow.Task{
		Name:         "Waiting until network is restored",
		Fn:           b.Shoot.Components.Extensions.Network.Wait,
		SkipIf:       b.Shoot.IsWorkerless,
		Dependencies: flow.NewTaskIDs(restoreNetwork),
	})

	reconcileOperatingSystemConfig := b.ReconcileOperatingSystemConfigTaskGroup(false)
	g = g.AddAll(reconcileOperatingSystemConfig.WithTaskDependencies(waitNetwork)...)

	// machine-controller-manager must run before the worker is restored so it adopts the migrated MachineDeployments.
	mcm := b.ReconcileMachineControllerManagerTaskGroup()
	g = g.AddAll(mcm.WithTaskDependencies(reconcileOperatingSystemConfig)...)
	worker := b.ReconcileWorkerTaskGroup(false)
	g = g.AddAll(worker.WithTaskDependencies(mcm)...)

	reconcileContainerRuntime := b.ReconcileContainerRuntimeTaskGroup(false)
	g = g.AddAll(reconcileContainerRuntime.WithTaskDependencies(worker)...)

	reconcileExtensionsAfterKubeAPIServer := b.ReconcileExtensionsAfterKubeAPIServerTaskGroup(false)
	g = g.AddAll(reconcileExtensionsAfterKubeAPIServer.WithTaskDependencies(reconcileContainerRuntime)...)

	// Deploy the permanent vpn-shoot and wait until it is healthy before tearing down the temporary VPN, so the
	// permanent tunnel to the destination is established first and connectivity is uninterrupted.
	deployVPNShoot := g.Add(flow.Task{
		Name:         "Deploying VPN shoot client to destination",
		Fn:           b.DeployVPNShoot,
		Dependencies: flow.NewTaskIDs(reconcileExtensionsAfterKubeAPIServer),
	})
	waitVPNShoot := g.Add(flow.Task{
		Name:         "Waiting until VPN shoot client to destination is healthy",
		Fn:           b.Shoot.Components.SystemComponents.VPNShoot.Wait,
		Dependencies: flow.NewTaskIDs(deployVPNShoot),
	})
	deployGardenerAccess := g.Add(flow.Task{
		Name:         "Deploying Gardener access resources",
		Fn:           b.Shoot.Components.GardenerAccess.Deploy,
		Dependencies: flow.NewTaskIDs(waitVPNShoot),
	})
	initializeShootClients := g.Add(flow.Task{
		Name:         "Initializing connection to shoot",
		Fn:           b.InitializeDesiredShootClients,
		Dependencies: flow.NewTaskIDs(deployGardenerAccess),
	})
	// Confirm the permanent tunnel actually carries traffic to the shoot BEFORE tearing down the
	// temporary VPN. Otherwise the temp path is removed while the permanent one may not yet be
	// established end-to-end (VPNShoot.Wait only checks pod/ManagedResource health, not tunnel
	// reachability), briefly black-holing apiserver->shoot traffic - webhooks, logs, exec.
	waitTunnelConnection := g.Add(flow.Task{
		Name:         "Waiting until the permanent VPN tunnel connection to the destination is established",
		Fn:           b.WaitUntilTunnelConnectionExists,
		Dependencies: flow.NewTaskIDs(initializeShootClients),
	})

	// TODO(GEP-39): Destroy the temporary VPN shoot client and live migration VPN DNS record after the permanent tunnel is established.

	// EnsureShootClusterIdentity populates status.ClusterIdentity, read by the system-components group, so it runs first.
	deployAPIServerProxy := g.Add(flow.Task{
		Name:         "Deploying apiserver-proxy",
		Fn:           b.DeployAPIServerProxy,
		SkipIf:       b.Shoot.IsWorkerless,
		Dependencies: flow.NewTaskIDs(waitTunnelConnection),
	})
	ensureShootClusterIdentity := g.Add(flow.Task{
		Name:         "Ensuring shoot cluster identity",
		Fn:           b.EnsureShootClusterIdentity,
		SkipIf:       b.Shoot.IsWorkerless,
		Dependencies: flow.NewTaskIDs(deployAPIServerProxy),
	})

	shootNamespaces := b.ReconcileShootNamespacesTaskGroup(false)
	g = g.AddAll(shootNamespaces.WithTaskDependencies(ensureShootClusterIdentity)...)

	systemComponents := b.ReconcileSystemComponentsTaskGroup(kubeProxyEnabled, false)
	g = g.AddAll(systemComponents.WithTaskDependencies(shootNamespaces)...)

	systemResources := b.ReconcileSystemResourcesTaskGroup()
	g = g.AddAll(systemResources.WithTaskDependencies(systemComponents)...)

	deployNodeAgent := g.Add(flow.Task{
		Name: "Deploying gardener-node-agent's OperatingSystemConfig secrets and RBAC resources",
		Fn: func(ctx context.Context) error {
			return b.DeployManagedResourceForGardenerNodeAgent(ctx, false)
		},
		SkipIf:       b.Shoot.IsWorkerless,
		Dependencies: flow.NewTaskIDs(systemResources),
	})
	populateStaticManifests := g.Add(flow.Task{
		Name:         "Populating static manifests from seed to shoot",
		Fn:           b.PopulateStaticManifestsFromSeedToShoot,
		Dependencies: flow.NewTaskIDs(deployNodeAgent),
	})
	updateAdvertisedAddresses := g.Add(flow.Task{
		Name:         "Ensuring advertised addresses for the shoot",
		Fn:           b.UpdateAdvertisedAddresses,
		Dependencies: flow.NewTaskIDs(populateStaticManifests),
	})

	deployLogging := g.Add(flow.Task{
		Name:         "Deploying shoot logging stack in seed",
		Fn:           b.DeployLogging,
		Dependencies: flow.NewTaskIDs(updateAdvertisedAddresses),
	})
	observabilityGroup := b.ReconcileObservabilityTaskGroup()
	g = g.AddAll(observabilityGroup.WithTaskDependencies(deployLogging)...)

	waitNginx := g.Add(flow.Task{
		Name:         "Waiting until nginx ingress LoadBalancer is ready",
		Fn:           b.WaitUntilNginxIngressServiceIsReady,
		SkipIf:       nginxDisabled,
		Dependencies: flow.NewTaskIDs(observabilityGroup),
	})
	_ = g.Add(flow.Task{
		Name:         "Deploying nginx ingress DNS record",
		Fn:           b.DeployOrDestroyIngressDNSRecord,
		SkipIf:       nginxDisabled,
		Dependencies: flow.NewTaskIDs(waitNginx),
	})

	return g
}

// taskGroupSourceSeedFinalTeardown is a flow.TaskID for a logical flow.TaskGroup.
const taskGroupSourceSeedFinalTeardown flow.TaskID = "TaskGroupSourceSeedFinalTeardown"

// sourceSeedFinalTeardownTaskGroup deletes the vpn-seed-server and the shoot namespace from the source seed.
// It runs after MigrationCompleted so the permanent vpn-shoot is already connected to the destination and
// vpn-shoot-tmp has already been destroyed. At that point the source vpn-seed-server has no active clients:
// tearing it down causes no reconnect loop and no shared kernel routing-table disruption.
func sourceSeedFinalTeardownTaskGroup(b *botanistpkg.Botanist) flow.TaskGroup {
	g := flow.NewTaskGroup(taskGroupSourceSeedFinalTeardown)
	destroyVPNServer := g.Add(flow.Task{
		Name: "Deleting vpn-seed-server from the source namespace",
		Fn:   b.DestroyVPNServer,
	})
	deleteNamespace := g.Add(flow.Task{
		Name:         "Deleting shoot namespace in Seed",
		Fn:           flow.TaskFn(b.DeleteSeedNamespace).RetryUntilTimeout(defaultInterval, defaultTimeout),
		Dependencies: flow.NewTaskIDs(destroyVPNServer),
	})
	_ = g.Add(flow.Task{
		Name:         "Waiting until shoot namespace in Seed has been deleted",
		Fn:           b.WaitUntilSeedNamespaceDeleted,
		Dependencies: flow.NewTaskIDs(deleteNamespace),
	})
	return g
}
