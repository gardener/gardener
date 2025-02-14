// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package create

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "create [token]",
		Short: "Create a bootstrap token on the server",
		Long: "The [token] is the actual token to write." +
			"This should be a securely generated random token of the form \"[a-z0-9]{6}.[a-z0-9]{16}\"." +
			"If no [token] is given, gardenadm will generate a random token instead.",

		Example: `# Create a bootstrap token with id "foo123" on the server
gardenadm token create foo123.bar4567890baz123

# Create a bootstrap token generated randomly
gardenadm token create`,

		Args: cobra.MaximumNArgs(1),

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
	opts.Log.Info("Not implemented")
	return nil
}
