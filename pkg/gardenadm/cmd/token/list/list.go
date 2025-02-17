// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package list

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all bootstrap tokens on the server",
		Long:  "List all bootstrap tokens on the server",

		Example: `# List all bootstrap tokens on the server
gardenadm token list`,

		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.Complete(); err != nil {
				return err
			}

			if err := opts.Validate(); err != nil {
				return err
			}

			return run(cmd.Context(), globalOpts, opts)
		},
	}

	opts.addFlags(cmd.Flags())

	return cmd
}

func run(_ context.Context, globalOpts *cmd.Options, _ *Options) error {
	fmt.Fprintln(globalOpts.IOStreams.Out, "not implemented")
	return nil
}
