// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package restore

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	initcmd "github.com/gardener/gardener/pkg/gardenadm/cmd/init"
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

	// The ETCD snapshot restored during `gardenadm restore` brings stale resources back to life which must be cleaned
	// up before the init flow re-reconciles the control plane. BootstrapControlPlane runs these cleanups (deleting the
	// prior control plane Node and its Pods, and the stale gardener-node-agent OperatingSystemConfig Secret) after the
	// connection to the control plane is established and before the bootstrap secrets are imported.
	b, err := initcmd.BootstrapControlPlane(ctx, initOpts, opts.BackupDataPath, opts.PriorNodeName, true)
	if err != nil {
		return fmt.Errorf("failed to bootstrap control plane (1st recovery phase): %w", err)
	}

	return initcmd.RunInitFlow(ctx, b, initOpts)
}
