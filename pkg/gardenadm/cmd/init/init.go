// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package init

import (
	"context"
	"fmt"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/gardener/gardener/pkg/gardenadm"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bootstrap the first control plane node",
		Long:  "Bootstrap the first control plane node",

		Example: `# Bootstrap the first control plane node
gardenadm init`,

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

func run(ctx context.Context, opts *Options) error {
	fs := afero.Afero{Fs: afero.NewOsFs()}

	cloudProfile, project, shoot, err := gardenadm.ReadKubernetesResourcesFromConfigDir(opts.Log, fs, opts.ConfigDir)
	if err != nil {
		return fmt.Errorf("failed reading Kubernetes resources from config directory %s: %w", opts.ConfigDir, err)
	}

	b, err := gardenadm.NewBotanist(ctx, opts.Log, project, cloudProfile, shoot)
	if err != nil {
		return fmt.Errorf("failed constructing botanist: %w", err)
	}

	var (
		g = flow.NewGraph("init")

		_ = g.Add(flow.Task{
			Name: "Initializing secrets management",
			Fn:   b.InitializeSecretsManagement,
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log: opts.Log,
	}); err != nil {
		return flow.Errors(err)
	}

	return nil
}
