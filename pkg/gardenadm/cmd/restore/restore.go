// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package restore

import (
	"context"

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
			Dependencies: flow.NewTaskIDs(deletePriorNode),
		})
		// TODO(ialidzhikov): Implement the required cleanups before running the init flow.
		// For more details, see https://github.com/gardener/gardener/issues/15279.
		_ = g.Add(flow.Task{
			Name: "Running init flow",
			Fn: func(ctx context.Context) error {
				return initcmd.RunInitFlow(ctx, b, initOpts)
			},
			Dependencies: flow.NewTaskIDs(deletePriorNode, forceDeletePriorNodePods),
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
