// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package restore

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	gardenadmbotanist "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	initcmd "github.com/gardener/gardener/pkg/gardenadm/cmd/init"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a control plane node from an etcd backup and manifest files",
		Long: `Restore a control plane node from an etcd backup and manifest files. Use this command to recover the
self-hosted shoot cluster's control plane node after a disaster (e.g., the control plane node is lost)
onto a new or existing node.`,

		Example: `# Restore a control plane node from an etcd backup
gardenadm restore --config-dir /path/to/manifests --backup-data-path /path/to/etcd-main/v2 --prior-node-name <name>`,

		Args: cobra.NoArgs,

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.ParseArgs(args); err != nil {
				return err
			}

			if err := opts.Validate(); err != nil {
				return err
			}

			if err := opts.Complete(); err != nil {
				return err
			}

			return run(cmd.Context(), opts)
		},
	}

	opts.addFlags(cmd.Flags())

	return cmd
}

func run(ctx context.Context, opts *Options) error {
	initOpts := &initcmd.Options{
		Options:         opts.Options,
		ManifestOptions: opts.ManifestOptions,
		// Restore requires an etcd backup, which only a control plane using etcd-druid (not the bootstrap etcd)
		// produces. Hence, restore always transitions to etcd-druid and does not expose --use-bootstrap-etcd.
		UseBootstrapEtcd: false,
		UseHostNetwork:   false,
		Zone:             opts.Zone,
	}

	// Track restore progress with a marker file so that a failed restore can be retried. The gardenlet deployment
	// recovered from the etcd snapshot would otherwise trip the guard in BootstrapControlPlane on a retry. We write the
	// marker before running the flow and remove it only on successful completion. While the marker exists, we reuse the
	// existing Force skip-path so the gardenlet check is skipped for this (retryable) restore.
	fs := afero.Afero{Fs: gardenadmbotanist.NewFs()}
	isRetry, err := prepareRestoreInProgressMarker(fs)
	if err != nil {
		return err
	}
	if isRetry {
		opts.Log.Info("Found restore-in-progress marker file, treating this invocation as a retry and skipping the existing-gardenlet check", "path", cmd.RestoreInProgressLocation)
		initOpts.Force = true
	}

	var (
		b *gardenadmbotanist.GardenadmBotanist

		g = flow.NewGraph("restore")

		bootstrapControlPlane = g.Add(flow.Task{
			Name: "Bootstrapping control plane",
			Fn: func(ctx context.Context) error {
				var err error
				b, err = initcmd.BootstrapControlPlane(ctx, initOpts, opts.BackupDataPath)
				return err
			},
		})
		deletePriorNode = g.Add(flow.Task{
			Name: "Deleting prior Node",
			Fn: func(ctx context.Context) error {
				return b.DeletePriorNode(ctx, opts.PriorNodeName)
			},
			Dependencies: flow.NewTaskIDs(bootstrapControlPlane),
		})
		forceDeletePriorNodePods = g.Add(flow.Task{
			Name: "Force deleting Pods running on prior Node",
			Fn: func(ctx context.Context) error {
				return b.ForceDeletePriorNodePods(ctx, opts.PriorNodeName)
			},
			Dependencies: flow.NewTaskIDs(bootstrapControlPlane),
		})
		// TODO(ialidzhikov): Implement the required cleanups before running the init flow.
		// For more details, see https://github.com/gardener/gardener/issues/15279.
		runInitFlow = g.Add(flow.Task{
			Name: "Running init flow",
			Fn: func(ctx context.Context) error {
				return initcmd.RunInitFlow(ctx, b, initOpts)
			},
			Dependencies: flow.NewTaskIDs(deletePriorNode, forceDeletePriorNodePods),
		})
		_ = g.Add(flow.Task{
			Name: "Marking restore as completed",
			Fn: func(_ context.Context) error {
				return removeRestoreInProgressMarker(opts.Log, b.FS)
			},
			Dependencies: flow.NewTaskIDs(runInitFlow),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log:              opts.Log,
		ProgressReporter: flow.NewCommandLineProgressReporter(opts.ErrOut),
	}); err != nil {
		return flow.Errors(err)
	}

	return nil
}

// prepareRestoreInProgressMarker ensures the restore-in-progress marker file exists. It returns true if the marker
// already existed before this invocation, indicating that this is a retry and the existing-gardenlet check should be
// skipped. Its presence is the only signal; the file content is not used.
func prepareRestoreInProgressMarker(fs afero.Afero) (bool, error) {
	markerExists, err := fs.Exists(cmd.RestoreInProgressLocation)
	if err != nil {
		return false, fmt.Errorf("failed checking whether restore-in-progress marker file %s exists: %w", cmd.RestoreInProgressLocation, err)
	}
	if markerExists {
		return true, nil
	}

	if err := fs.WriteFile(cmd.RestoreInProgressLocation, nil, 0640); err != nil {
		return false, fmt.Errorf("failed writing restore-in-progress marker file %s: %w", cmd.RestoreInProgressLocation, err)
	}
	return false, nil
}

// removeRestoreInProgressMarker removes the restore-in-progress marker file. Only this final restore task removes the
// marker, so it is expected to exist at this point. If it is already gone, something removed it out of band; we do not
// fail the (otherwise successful) restore over it, but log a warning so the anomaly is visible.
func removeRestoreInProgressMarker(log logr.Logger, fs afero.Afero) error {
	err := fs.Remove(cmd.RestoreInProgressLocation)
	if err == nil {
		return nil
	}
	if errors.Is(err, afero.ErrFileNotFound) || os.IsNotExist(err) {
		log.Info("Warning: restore-in-progress marker file was already absent at completion; something removed it out of band", "path", cmd.RestoreInProgressLocation)
		return nil
	}
	return fmt.Errorf("failed removing restore-in-progress marker file %s: %w", cmd.RestoreInProgressLocation, err)
}
