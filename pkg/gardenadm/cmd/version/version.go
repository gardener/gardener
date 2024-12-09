// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/component-base/version"
)

// NewCommand creates a new cobra.Command.
func NewCommand(ioStreams genericiooptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the client version information",
		Long:  "Print the client version information",

		Run: func(_ *cobra.Command, _ []string) {
			fmt.Fprintf(ioStreams.Out, "gardenadm version %s\n", version.Get())
		},
	}

	return cmd
}
