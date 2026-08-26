// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/component-base/version"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// NewCommand creates a new cobra.Command.
func NewCommand(opts *cmd.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the client version information",
		Long:  "Print the client version information",

		Args: cobra.NoArgs,

		Run: func(_ *cobra.Command, _ []string) {
			fmt.Fprintf(opts.Out, "gardenadm version %s\n", version.Get())
		},
	}

	return cmd
}
