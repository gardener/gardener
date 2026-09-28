// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/flow"
	shootstate "github.com/gardener/gardener/pkg/utils/gardener/shootstate"
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
			return retryutils.UntilTimeout(ctx, 10*time.Second, 10*time.Minute, func(context.Context) (done bool, err error) {
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

	var (
		g = flow.NewGraph("Shoot control plane live migration")

		sourceEtcdReadyForPeerJoin = g.Add(flow.Task{
			Name: "Making source etcd ready for peer join",
			Fn: r.executeStepOrWait(botanist, role, gardencorev1beta1.ShootLiveMigrationSourceEtcdPreparedForPeerJoin, func(ctx context.Context, b *botanistpkg.Botanist) error {
				if err := shootstate.Deploy(ctx, b.Clock, b.GardenClient, b.SeedClientSet.Client(),
					b.Shoot.GetInfo(), b.Shoot.ControlPlaneNamespace, false); err != nil {
					return fmt.Errorf("failed to persist shoot state: %w", err)
				}
				if err := b.DeployEtcdPeerExposure(ctx); err != nil {
					return fmt.Errorf("failed to deploy etcd peer exposure: %w", err)
				}
				if err := b.InitializeSecretsManagement(ctx); err != nil {
					return fmt.Errorf("failed to initialize secrets management: %w", err)
				}
				if err := b.DeployEtcd(ctx); err != nil {
					return fmt.Errorf("failed to deploy etcd: %w", err)
				}
				if err := b.WaitUntilEtcdsReady(ctx); err != nil {
					return fmt.Errorf("failed to wait until etcds are ready: %w", err)
				}
				if err := b.Shoot.Components.BackupEntry.Migrate(ctx); err != nil {
					return fmt.Errorf("failed to migrate backup entry: %w", err)
				}
				return b.Shoot.Components.BackupEntry.WaitMigrate(ctx)
			}),
		})

		_ = g.Add(flow.Task{
			Name: "Joining destination etcd to the source cluster",
			Fn: r.executeStepOrWait(botanist, role, gardencorev1beta1.ShootLiveMigrationDestinationEtcdPeersJoined, func(ctx context.Context, b *botanistpkg.Botanist) error {
				if err := b.DeployControlPlaneNamespace(ctx); err != nil {
					return fmt.Errorf("failed to deploy control plane namespace: %w", err)
				}
				if err := b.InitializeSecretsManagement(ctx); err != nil {
					return fmt.Errorf("failed to initialize secrets management: %w", err)
				}
				if err := b.DeployEtcdPeerExposure(ctx); err != nil {
					return fmt.Errorf("failed to deploy etcd peer exposure: %w", err)
				}
				// The source backup entry is deployed to ensure that the data in the source seed's backup bucket
				// is properly cleaned up at a later stage of the flow.
				if err := b.DeploySourceBackupEntry(ctx); err != nil {
					return fmt.Errorf("failed to deploy source backup entry: %w", err)
				}
				if err := b.Shoot.Components.SourceBackupEntry.Wait(ctx); err != nil {
					return fmt.Errorf("failed to wait for source backup entry: %w", err)
				}
				if err := b.Shoot.Components.BackupEntry.Restore(ctx, nil); err != nil {
					return fmt.Errorf("failed to deploy backup entry: %w", err)
				}
				if err := b.Shoot.Components.BackupEntry.Wait(ctx); err != nil {
					return fmt.Errorf("failed to wait for backup entry: %w", err)
				}
				if err := b.DeployEtcd(ctx); err != nil {
					return fmt.Errorf("failed to deploy etcd: %w", err)
				}
				return b.WaitUntilEtcdsReady(ctx)
			}),
			Dependencies: flow.NewTaskIDs(sourceEtcdReadyForPeerJoin),
		})

		// TODO(GEP-39): Future PRs will add other steps as the topic progresses.
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

func (r *Reconciler) executeStepOrWait(botanist *botanistpkg.Botanist, role v1beta1helper.LiveMigrationRole, conditionType gardencorev1beta1.ConditionType, fn func(ctx context.Context, b *botanistpkg.Botanist) error) flow.TaskFn {
	owner := liveMigrationStepOwners[conditionType]
	return flow.TaskFn(func(ctx context.Context) error {
		if role != owner {
			return r.waitForLiveMigrationPeerStep(ctx, botanist, conditionType)
		}
		if err := r.setLiveMigrationStepCondition(ctx, botanist.Shoot.GetInfo(), conditionType, false); err != nil {
			return err
		}
		if fn != nil {
			if err := fn(ctx, botanist); err != nil {
				if conditionErr := r.setLiveMigrationStepConditionError(ctx, botanist.Shoot.GetInfo(), conditionType, err); conditionErr != nil {
					botanist.Logger.Error(conditionErr, "Failed to set error condition for live migration step", "step", conditionType)
				}
				return err
			}
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

func (r *Reconciler) patchLiveMigrationConditions(ctx context.Context, shoot *gardencorev1beta1.Shoot, conditions ...gardencorev1beta1.Condition) error {
	patch := client.StrategicMergeFrom(shoot.DeepCopy())

	if shoot.Status.LiveMigration == nil {
		shoot.Status.LiveMigration = &gardencorev1beta1.LiveMigration{}
	}
	shoot.Status.LiveMigration.Conditions = v1beta1helper.MergeConditions(shoot.Status.LiveMigration.Conditions, conditions...)

	return r.GardenClient.Status().Patch(ctx, shoot, patch)
}
