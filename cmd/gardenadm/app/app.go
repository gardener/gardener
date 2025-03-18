// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/bootstrap"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/connect"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/discover"
	initcmd "github.com/gardener/gardener/pkg/gardenadm/cmd/init"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/join"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/token"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/version"
)

// Name is a const for the name of this component.
const Name = "gardenadm"

// NewCommand creates a new cobra.Command for running gardenadm.
func NewCommand() *cobra.Command {
	opts := &cmd.Options{
		IOStreams: genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}

	cmd := &cobra.Command{
		Use:   Name,
		Short: Name + " bootstraps and manages autonomous shoot clusters in the Gardener project.",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if err := opts.Validate(); err != nil {
				return err
			}

			if err := opts.Complete(); err != nil {
				return err
			}

			return nil
		},
	}

	// don't output usage on further errors raised during execution
	cmd.SilenceUsage = true

	opts.AddFlags(cmd.PersistentFlags())

	prepareClusterBootstrapGroup(cmd, opts)
	prepareGardenGroup(cmd, opts)
	prepareAdditionalGroup(cmd, opts)

	return cmd
}

func prepareClusterBootstrapGroup(cmd *cobra.Command, opts *cmd.Options) {
	group := &cobra.Group{
		ID:    "cluster-bootstrap",
		Title: "Autonomous Shoot Cluster Bootstrap Commands:",
	}
	cmd.AddGroup(group)

	for _, subcommand := range []*cobra.Command{
		initcmd.NewCommand(opts),
		join.NewCommand(opts),
		bootstrap.NewCommand(opts),
		token.NewCommand(opts),
	} {
		subcommand.GroupID = group.ID
		cmd.AddCommand(subcommand)
	}
}

func prepareGardenGroup(cmd *cobra.Command, opts *cmd.Options) {
	group := &cobra.Group{
		ID:    "garden",
		Title: "Garden Cluster Interaction Commands:",
	}
	cmd.AddGroup(group)

	for _, subcommand := range []*cobra.Command{
		discover.NewCommand(opts),
		connect.NewCommand(opts),
	} {
		subcommand.GroupID = group.ID
		cmd.AddCommand(subcommand)
	}
}

func prepareAdditionalGroup(cmd *cobra.Command, opts *cmd.Options) {
	group := &cobra.Group{
		ID:    "additional",
		Title: "Additional Commands:",
	}
	cmd.AddGroup(group)
	cmd.SetHelpCommandGroupID(group.ID)
	cmd.SetCompletionCommandGroupID(group.ID)

	for _, subcommand := range []*cobra.Command{
		version.NewCommand(opts),
	} {
		subcommand.GroupID = group.ID
		cmd.AddCommand(subcommand)
	}
}
