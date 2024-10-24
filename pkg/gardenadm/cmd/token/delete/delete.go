// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package delete

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
		Use:   "delete [token-id]",
		Short: "Delete a bootstrap token on the server",
		Long: "This command will delete a bootstrap token for you." +
			"The [token-id] is the ID of the token of the form \"[a-z0-9]{6}\" to delete",

		Example: `# Delete a bootstrap token with id "foo123" on the server
gardenadm token delete foo123`,

		Args: cobra.MaximumNArgs(1),

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(args); err != nil {
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
	fmt.Fprint(ioStreams.Out, "not implemented")
	return nil
}
