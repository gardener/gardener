// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package join

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "join",
		Short: "Bootstrap worker nodes and join them to the cluster",
		Long: `Bootstrap worker nodes and join them to the cluster.

This command helps to initialize and configure a node to join an existing autonomous shoot cluster.
It ensures that the necessary configurations are applied and the node is properly registered as a worker or control plane node.

Note that further control plane nodes cannot be joined currently.`,
		Example: `# Bootstrap a worker node and join it to the cluster
gardenadm join --bootstrap-token <token> --ca-certificate <ca-cert> --gardener-node-agent-secret-name <secret-name> <control-plane-address>`,

		Args: cobra.ExactArgs(1),

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

func run(_ context.Context, opts *Options) error {
	opts.Log.Info("Not implemented either")
	return nil
}
