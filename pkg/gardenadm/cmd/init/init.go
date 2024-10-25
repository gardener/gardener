// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package init

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
		Use:   "init",
		Short: "Bootstrap the first control plane node",
		Long:  "Bootstrap the first control plane node",

		Example: `# Bootstrap the first control plane node
gardenadm init`,

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
	fmt.Fprint(ioStreams.Out, "not implemented")
	return nil
}
