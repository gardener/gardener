// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
)

// NewCommand creates a new cobra.Command.
func NewCommand(ioStreams genericiooptions.IOStreams) *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the infrastructure for an Autonomous Shoot Cluster",
		Long:  "Bootstrap the infrastructure for an Autonomous Shoot Cluster (networks, machines, etc.)",

		Example: `# Bootstrap the infrastructure
gardenadm bootstrap --kubeconfig ~/.kube/config`,

		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.Complete(); err != nil {
				return err
			}

			if err := opts.Validate(); err != nil {
				return err
			}

			return run(cmd.Context(), ioStreams, opts)
		},
	}

	opts.addFlags(cmd.Flags())

	return cmd
}

func run(_ context.Context, ioStreams genericiooptions.IOStreams, _ *Options) error {
	fmt.Fprintln(ioStreams.Out, "not implemented")
	return nil
}
