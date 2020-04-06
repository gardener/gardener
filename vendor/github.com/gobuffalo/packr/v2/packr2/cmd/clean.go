package cmd

import (
	"github.com/gobuffalo/packr/v2/jam"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "removes any *-packr.go files",
	RunE: func(cmd *cobra.Command, args []string) error {
		return jam.Clean(args...)
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}
