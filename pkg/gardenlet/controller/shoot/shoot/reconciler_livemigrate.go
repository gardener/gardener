// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"k8s.io/utils/ptr"
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
	gardencorev1beta1.ShootLiveMigrationSourceSeedCleanup:                          v1beta1helper.LiveMigrationRoleSource,
	gardencorev1beta1.ShootLiveMigrationEtcdMigrationComplete:                      v1beta1helper.LiveMigrationRoleDestination,
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
	gardencorev1beta1.ShootLiveMigrationSourceSeedCleanup,
	gardencorev1beta1.ShootLiveMigrationEtcdMigrationComplete,
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

		destinationEtcdPeersJoined = g.Add(flow.Task{
			Name:         "Joining destination etcd to the source cluster",
			Fn:           r.executeStepOrWait(botanist, role, gardencorev1beta1.ShootLiveMigrationDestinationEtcdPeersJoined, destinationEtcdPeersJoinedTaskGroup(botanist)),
			Dependencies: flow.NewTaskIDs(sourceEtcdReadyForPeerJoin),
		})

		_ = g.Add(flow.Task{
			Name:         "Migrating etcd cluster to destination seed",
			Fn:           r.executeStepOrWait(botanist, role, gardencorev1beta1.ShootLiveMigrationEtcdMigrationComplete, etcdMigrationCompleteTaskGroup(botanist)),
			Dependencies: flow.NewTaskIDs(destinationEtcdPeersJoined),
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

// executeStepOrWait returns a TaskFn that runs the given task group if this gardenlet owns the step, or waits for the peer gardenlet to complete it.
func (r *Reconciler) executeStepOrWait(botanist *botanistpkg.Botanist, role v1beta1helper.LiveMigrationRole, conditionType gardencorev1beta1.ConditionType, group flow.TaskGroup) flow.TaskFn {
	owner := liveMigrationStepOwners[conditionType]
	return flow.TaskFn(func(ctx context.Context) error {
		if err := r.checkGardenletVersionParity(ctx, botanist.Shoot.GetInfo(), role); err != nil {
			return err
		}
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

	// Refresh the cached shoot so the next patchLiveMigrationConditions call has all conditions
	// (including those set by the peer gardenlet) in the local merge, preserving sort order.
	botanist.Shoot.SetInfo(shoot)
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
	// Use an empty conditions list as the strategic-merge-patch base so that ALL merged conditions appear in the patch payload in sorted order.
	// A minimal diff (only the changed item) causes the server to re-order the list: the patched item floats to the front while unchanged conditions are appended as "server-only" items,
	// breaking the canonical flow order. Sending the full sorted list makes the server apply them in patch order, which matches liveMigrationConditionOrder.
	base := shoot.DeepCopy()
	base.Status.LiveMigration = &gardencorev1beta1.LiveMigration{}
	patch := client.StrategicMergeFrom(base)

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
		g = flow.NewTaskGroup(taskGroupSourceEtcdPreparedForPeerJoin).
			SkipIf(v1beta1helper.GetLiveMigrationCondition(b.Shoot.GetInfo(), gardencorev1beta1.ShootLiveMigrationSourceSeedCleanup) != nil)

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
		isDestinationJoined = v1beta1helper.GetLiveMigrationCondition(b.Shoot.GetInfo(), gardencorev1beta1.ShootLiveMigrationDestinationEtcdPeersJoined) != nil

		migrateBackupEntry = g.Add(flow.Task{
			Name:         "Migrating backup entry",
			Fn:           b.Shoot.Components.BackupEntry.Migrate,
			SkipIf:       isDestinationJoined,
			Dependencies: flow.NewTaskIDs(waitEtcdsReady),
		})
		_ = g.Add(flow.Task{
			Name:         "Waiting until backup entry has been migrated",
			Fn:           b.Shoot.Components.BackupEntry.WaitMigrate,
			SkipIf:       isDestinationJoined,
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
		g = flow.NewTaskGroup(taskGroupDestinationEtcdPeersJoined).
			SkipIf(v1beta1helper.GetLiveMigrationCondition(b.Shoot.GetInfo(), gardencorev1beta1.ShootLiveMigrationEtcdMigrationComplete) != nil)

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
			Name: "Deploying destination etcd to join the source etcd cluster",
			Fn: func(ctx context.Context) error {
				return b.DeployEtcdForLiveMigrationPhase(ctx, 2)
			},
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
	g := flow.NewTaskGroup(taskGroupDestinationKubeAPIServerReady).
		SkipIf(v1beta1helper.GetLiveMigrationCondition(b.Shoot.GetInfo(), gardencorev1beta1.ShootLiveMigrationMigrationCompleted) != nil)

	refreshShootState := g.Add(flow.Task{
		Name: "Refreshing shoot state",
		Fn:   b.RefreshShootStateForLiveMigration,
	})
	deployVPNServer := g.Add(flow.Task{
		Name:         "Deploying destination VPN server",
		Fn:           b.DeployVPNServer,
		SkipIf:       b.Shoot.IsWorkerless,
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
	g = g.AddAll(reconcileExtensions.TasksWithDependencies(reconcileIstioConfigMap)...)

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
	g = g.AddAll(reconcileKubeAPIServer.TasksWithDependencies(waitService, deployVPNServer)...)

	reconcileGardenerResourceManager := b.ReconcileGardenerResourceManagerTaskGroup(true, false)
	g = g.AddAll(reconcileGardenerResourceManager.TasksWithDependencies(reconcileKubeAPIServer)...)

	reconcileControlPlaneComponents := b.DeployControlPlaneComponentsTaskGroup(false)
	g = g.AddAll(reconcileControlPlaneComponents.TasksWithDependencies(reconcileGardenerResourceManager)...)

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

// taskGroupSourceSeedCleanup is a flow.TaskID for a logical flow.TaskGroup.
const taskGroupSourceSeedCleanup flow.TaskID = "TaskGroupSourceSeedCleanup"

// sourceSeedCleanupTaskGroup migrates and tears down the source-side shoot resources (extensions, infrastructure,
// control plane, managed resources, machines) and persists the migrated state for the destination to restore.
func sourceSeedCleanupTaskGroup(b *botanistpkg.Botanist) flow.TaskGroup {
	var (
		g = flow.NewTaskGroup(taskGroupSourceSeedCleanup).
			SkipIf(v1beta1helper.GetLiveMigrationCondition(b.Shoot.GetInfo(), gardencorev1beta1.ShootLiveMigrationEtcdMigrationComplete) != nil)
	)

	migrateDNSRecordsTaskGroup := b.MigrateDNSRecordsTaskGroup()
	g = g.AddAll(migrateDNSRecordsTaskGroup.TasksWithDependencies()...)

	migrateManagedResources := b.MigrateAndDeleteManagedResourcesTaskGroup(true)
	g = g.AddAll(migrateManagedResources.TasksWithDependencies(migrateDNSRecordsTaskGroup)...)

	deleteMachineControllerManager := b.DeleteMachineControllerManagerTaskGroup()
	g = g.AddAll(deleteMachineControllerManager.TasksWithDependencies(migrateManagedResources)...)

	migrateExtensions := b.MigrateExtensionResourcesTaskGroup()
	g = g.AddAll(migrateExtensions.TasksWithDependencies(deleteMachineControllerManager)...)

	persistShootState := g.Add(flow.Task{
		Name:         "Persisting shoot state",
		Fn:           b.PersistShootState,
		Dependencies: flow.NewTaskIDs(migrateExtensions),
	})

	destroyExtensionsAndMachineResources := b.DestroyExtensionsAndMachineResourcesTaskGroup()
	g = g.AddAll(destroyExtensionsAndMachineResources.TasksWithDependencies(persistShootState)...)

	migrateControlPlane := b.MigrateAndDeleteControlPlaneTaskGroup()
	g = g.AddAll(migrateControlPlane.TasksWithDependencies(destroyExtensionsAndMachineResources)...)

	migrateExtensionsAfterKubeAPIServerGroup := b.MigrateAndDeleteExtensionsAfterKubeAPIServerTaskGroup()
	g = g.AddAll(migrateExtensionsAfterKubeAPIServerGroup.TasksWithDependencies(migrateControlPlane)...)

	_ = g.Add(flow.Task{
		Name:         "Waiting until all extensions have been deleted",
		Fn:           b.Shoot.Components.Extensions.Extension.WaitCleanup,
		Dependencies: flow.NewTaskIDs(migrateExtensionsAfterKubeAPIServerGroup),
	})

	migrateInfrastructureGroup := b.MigrateAndDeleteInfrastructureTaskGroup()
	g = g.AddAll(migrateInfrastructureGroup.TasksWithDependencies(migrateExtensionsAfterKubeAPIServerGroup)...)

	return g
}

// taskGroupEtcdMigrationComplete is a flow.TaskID for the etcd migration completion task group.
const taskGroupEtcdMigrationComplete flow.TaskID = "TaskGroupEtcdMigrationComplete"

// etcdMigrationCompleteTaskGroup returns the flow.TaskGroup for completing the etcd migration.
// It first deploys all destination DNS records and waits for DNS caches to propagate, then scales
// out the destination etcd to 3 replicas, removes source members, cleans up the live migration
// configuration, and takes a full snapshot.
// DNS is deployed here — before source etcd members are removed — so the source kube-apiserver can
// only go down after all DNS caches have already switched to the destination.
func etcdMigrationCompleteTaskGroup(b *botanistpkg.Botanist) flow.TaskGroup {
	var (
		skipIfSourceMembersAlreadyRemoved = func(fn flow.TaskFn) flow.TaskFn {
			return func(ctx context.Context) error {
				sourceMembersAlreadyRemoved, err := b.IsSourceMembersAlreadyRemoved(ctx)
				if err != nil || sourceMembersAlreadyRemoved {
					return err
				}
				return fn(ctx)
			}
		}

		g = flow.NewTaskGroup(taskGroupEtcdMigrationComplete).
			SkipIf(v1beta1helper.GetLiveMigrationCondition(b.Shoot.GetInfo(), gardencorev1beta1.ShootLiveMigrationSourceSeedFinalTeardown) != nil)
	)

	// Deploy all DNS records pointing to the destination before scaling out etcd, so the source
	// kube-apiserver can only go down after DNS records have already switched to the destination.
	deployDNS := b.ReconcileDNSRecordsTaskGroup()
	g = g.AddAll(deployDNS.TasksWithDependencies()...)

	waitDNSCacheSync := g.Add(flow.Task{
		Name:         "Waiting for DNS caches to sync to the destination",
		Fn:           skipIfSourceMembersAlreadyRemoved(b.WaitForLiveMigrationDNSCacheSync),
		Dependencies: flow.NewTaskIDs(deployDNS),
	})
	deployEtcd := g.Add(flow.Task{
		Name:         "Scaling out destination etcd to 3 replicas",
		Fn:           skipIfSourceMembersAlreadyRemoved(b.DeployEtcd),
		Dependencies: flow.NewTaskIDs(waitDNSCacheSync),
	})
	waitUntilEtcdReady := g.Add(flow.Task{
		Name:         "Waiting until etcd is scaled out and ready",
		Fn:           skipIfSourceMembersAlreadyRemoved(b.WaitUntilEtcdsReady),
		Dependencies: flow.NewTaskIDs(deployEtcd),
	})
	removeSourceMembers := g.Add(flow.Task{
		Name:         "Removing source etcd members from the joined cluster",
		Fn:           skipIfSourceMembersAlreadyRemoved(b.RemoveSourceEtcdMembers),
		Dependencies: flow.NewTaskIDs(waitUntilEtcdReady),
	})
	waitUntilSourceMembersRemoved := g.Add(flow.Task{
		Name:         "Waiting until source etcd members are removed",
		Fn:           b.WaitUntilSourceEtcdMembersRemoved,
		Dependencies: flow.NewTaskIDs(removeSourceMembers),
	})
	deployEtcdWithoutLiveMigration := g.Add(flow.Task{
		Name: "Removing live migration configuration from etcd",
		Fn: func(ctx context.Context) error {
			b.Shoot.Components.ControlPlane.EtcdMain.SetLiveMigration(nil)
			return b.DeployEtcd(ctx)
		},
		Dependencies: flow.NewTaskIDs(waitUntilSourceMembersRemoved),
	})
	waitUntilEtcdReadyAfterCleanup := g.Add(flow.Task{
		Name:         "Waiting until etcds are ready after cleanup",
		Fn:           b.WaitUntilEtcdsReady,
		Dependencies: flow.NewTaskIDs(deployEtcdWithoutLiveMigration),
	})
	_ = g.Add(flow.Task{
		Name:         "Destroying ETCD member exposure",
		Fn:           b.DestroyEtcdMemberExposure,
		Dependencies: flow.NewTaskIDs(waitUntilEtcdReadyAfterCleanup),
	})
	_ = g.Add(flow.Task{
		Name:         "Taking full etcd snapshot",
		Fn:           b.SnapshotEtcd,
		SkipIf:       v1beta1helper.GetBackupConfigForShoot(b.Shoot.GetInfo(), b.GetSeed()) == nil,
		Dependencies: flow.NewTaskIDs(waitUntilEtcdReadyAfterCleanup),
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
	g = g.AddAll(deployReferencedResources.TasksWithDependencies(refreshShootState)...)

	deployCloudProviderSecret := b.DeployCloudProviderSecretTaskGroup()
	g = g.AddAll(deployCloudProviderSecret.TasksWithDependencies(deployReferencedResources)...)

	reconcileInfrastructure := b.ReconcileInfrastructureTaskGroup(false)
	g = g.AddAll(reconcileInfrastructure.TasksWithDependencies(deployCloudProviderSecret)...)

	reconcileControlPlane := b.ReconcileControlPlaneTaskGroup(false)
	g = g.AddAll(reconcileControlPlane.TasksWithDependencies(reconcileInfrastructure)...)

	restoreNetwork := g.Add(flow.Task{
		Name:         "Restoring network",
		Fn:           b.DeployNetwork,
		SkipIf:       b.Shoot.IsWorkerless,
		Dependencies: flow.NewTaskIDs(reconcileControlPlane),
	})
	waitNetwork := g.Add(flow.Task{
		Name:         "Waiting until network is restored",
		Fn:           flow.TaskFn(func(ctx context.Context) error { return b.Shoot.Components.Extensions.Network.Wait(ctx) }),
		SkipIf:       b.Shoot.IsWorkerless,
		Dependencies: flow.NewTaskIDs(restoreNetwork),
	})

	reconcileOperatingSystemConfig := b.ReconcileOperatingSystemConfigTaskGroup()
	g = g.AddAll(reconcileOperatingSystemConfig.TasksWithDependencies(waitNetwork)...)

	// machine-controller-manager must run before the worker is restored so it adopts the migrated MachineDeployments.
	mcm := b.ReconcileMachineControllerManagerTaskGroup()
	g = g.AddAll(mcm.TasksWithDependencies(reconcileOperatingSystemConfig)...)
	worker := b.ReconcileWorkerTaskGroup(false)
	g = g.AddAll(worker.TasksWithDependencies(mcm)...)

	reconcileContainerRuntime := b.ReconcileContainerRuntimeTaskGroup(false)
	g = g.AddAll(reconcileContainerRuntime.TasksWithDependencies(worker)...)

	reconcileExtensionsAfterKubeAPIServer := b.ReconcileExtensionsAfterKubeAPIServerTaskGroup(false)
	g = g.AddAll(reconcileExtensionsAfterKubeAPIServer.TasksWithDependencies(reconcileContainerRuntime)...)

	// Deploy the permanent vpn-shoot and wait until it is healthy before tearing down the temporary VPN, so the
	// permanent tunnel to the destination is established first and connectivity is uninterrupted.
	deployVPNShoot := g.Add(flow.Task{
		Name:         "Deploying VPN shoot client to destination",
		Fn:           b.DeployVPNShoot,
		SkipIf:       b.Shoot.IsWorkerless,
		Dependencies: flow.NewTaskIDs(reconcileExtensionsAfterKubeAPIServer),
	})
	waitVPNShoot := g.Add(flow.Task{
		Name:         "Waiting until VPN shoot client to destination is healthy",
		Fn:           flow.TaskFn(func(ctx context.Context) error { return b.Shoot.Components.SystemComponents.VPNShoot.Wait(ctx) }),
		SkipIf:       b.Shoot.IsWorkerless,
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
		SkipIf:       b.Shoot.IsWorkerless,
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
	g = g.AddAll(shootNamespaces.TasksWithDependencies(ensureShootClusterIdentity)...)

	systemComponents := b.ReconcileSystemComponentsTaskGroup(kubeProxyEnabled, false)
	g = g.AddAll(systemComponents.TasksWithDependencies(shootNamespaces)...)

	systemResources := b.ReconcileSystemResourcesTaskGroup()
	g = g.AddAll(systemResources.TasksWithDependencies(systemComponents)...)

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
	g = g.AddAll(observabilityGroup.TasksWithDependencies(deployLogging)...)

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

// checkGardenletVersionParity returns an error when the local and peer gardenlet versions differ or are not reported.
func (r *Reconciler) checkGardenletVersionParity(ctx context.Context, shoot *gardencorev1beta1.Shoot, role v1beta1helper.LiveMigrationRole) error {
	if r.Config.SeedConfig == nil {
		return fmt.Errorf("seed config is not set")
	}
	ownSeedName := r.Config.SeedConfig.Name

	localVersion := r.Identity.Version
	if localVersion == "" {
		return newGardenletVersionNotReportedError(role, ownSeedName)
	}

	peerSeedName, peerRole := liveMigratePeer(shoot, ownSeedName)
	if peerRole == v1beta1helper.LiveMigrationRoleNone {
		// The shoot is no longer undergoing a live migration (e.g. it was aborted).
		return nil
	}

	peerSeed := &gardencorev1beta1.Seed{}
	if err := r.GardenAPIReader.Get(ctx, client.ObjectKey{Name: peerSeedName}, peerSeed); err != nil {
		return fmt.Errorf("getting live migration peer seed %q: %w", peerSeedName, err)
	}

	peerVersion := ptr.Deref(peerSeed.Status.Gardener, gardencorev1beta1.Gardener{}).Version
	if peerVersion == "" {
		return newGardenletVersionNotReportedError(peerRole, peerSeedName)
	}

	if localVersion != peerVersion {
		sourceVersion, destinationVersion := localVersion, peerVersion
		if role == v1beta1helper.LiveMigrationRoleDestination {
			sourceVersion, destinationVersion = destinationVersion, sourceVersion
		}
		return fmt.Errorf("source and destination seeds must run the same gardenlet version (source: %q, destination: %q)", sourceVersion, destinationVersion)
	}

	return nil
}

// newGardenletVersionNotReportedError returns an error for a seed whose gardenlet version is not reported.
func newGardenletVersionNotReportedError(role v1beta1helper.LiveMigrationRole, seedName string) error {
	return fmt.Errorf("gardenlet version not reported for %s seed %q", strings.ToLower(string(role)), seedName)
}

func liveMigratePeer(shoot *gardencorev1beta1.Shoot, ownSeedName string) (seedName string, role v1beta1helper.LiveMigrationRole) {
	switch v1beta1helper.GetLiveMigrationRole(shoot, ownSeedName) {
	case v1beta1helper.LiveMigrationRoleSource:
		return ptr.Deref(shoot.Spec.SeedName, ""), v1beta1helper.LiveMigrationRoleDestination
	case v1beta1helper.LiveMigrationRoleDestination:
		return ptr.Deref(shoot.Status.SeedName, ""), v1beta1helper.LiveMigrationRoleSource
	default:
		return "", v1beta1helper.LiveMigrationRoleNone
	}
}
