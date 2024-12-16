// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	gardenadm "github.com/gardener/gardener/cmd/gardenadm/app"
)

var commands = map[string]*cobra.Command{
	"gardenadm": gardenadm.NewCommand(),
}

func main() {
	var outputDir string

	cmd := &cobra.Command{
		Use: "cli-reference-generator [-O output-dir] binary",

		ValidArgs: slices.Collect(maps.Keys(commands)),
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),

		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true

			binary := args[0]
			fmt.Printf("Generating CLI reference docs for %s to %s\n", binary, outputDir)

			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return err
			}

			command := commands[binary]
			command.DisableAutoGenTag = true

			return doc.GenMarkdownTree(command, outputDir)
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output-dir", "O", ".", "Directory to write output to")

	if err := cmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
