package cmd

import (
	"github.com/gobuffalo/packr/v2/jam"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:                "install",
	Short:              "Wraps the go install command with packr",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cargs := parseArgs(args)
		if err := jam.Pack(globalOptions.PackOptions); err != nil {
			return err
		}
		return goCmd("install", cargs...)
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}
