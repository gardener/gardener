// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	opts := &Options{}

	cmd := &cobra.Command{
		Use: "extension-generator",

		Short: "Generate an Extension (operator.gardener.cloud) manifest.",

		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true

			if err := opts.Validate(); err != nil {
				return err
			}

			extensionYaml, err := GenerateExtension(opts)
			if err != nil {
				return err
			}

			return os.WriteFile(opts.Destination, extensionYaml, 0644)
		},
	}

	opts.AddFlags(cmd.Flags())

	if err := cmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
