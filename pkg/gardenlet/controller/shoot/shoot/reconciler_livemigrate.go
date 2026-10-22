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

		_ = g.Add(flow.Task{
			Name:         "Migrating source extensions needed before kube-apiserver",
			Fn:           r.executeStepOrWait(botanist, role, gardencorev1beta1.ShootLiveMigrationMigrateExtensionsNeededBeforeKubeAPIServer, botanist.MigrateExtensionsBeforeKubeAPIServerTaskGroup()),
			Dependencies: flow.NewTaskIDs(destinationEtcdJoined),
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
