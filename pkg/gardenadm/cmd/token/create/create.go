// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package create

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	tokenutils "github.com/gardener/gardener/pkg/gardenadm/cmd/token/utils"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{
		Options:       globalOpts,
		CreateOptions: &tokenutils.Options{Options: globalOpts},
	}

	cmd := &cobra.Command{
		Use:   "create [token]",
		Short: "Create a bootstrap token on the server for joining a node",
		Long: `The [token] is the bootstrap token to be created on the server.
This token is used for securely authenticating nodes or clients to the cluster.
It must follow the format "[a-z0-9]{6}.[a-z0-9]{16}" to ensure compatibility with Kubernetes bootstrap token requirements.
If no [token] is provided, gardenadm will automatically generate a secure random token for you.`,

		Example: `# Create a bootstrap token with a specific ID and secret
gardenadm token create foo123.bar4567890baz123

# Create a bootstrap token with a specific ID and secret and directly print the gardenadm join command
gardenadm token create foo123.bar4567890baz123 --print-join-command

# Generate a random bootstrap token for joining a node
gardenadm token create`,

		Args: cobra.MaximumNArgs(1),

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
	clientSet, err := tokenutils.CreateClientSet(ctx, opts.Log)
	if err != nil {
		return fmt.Errorf("failed creating client set: %w", err)
	}

	split := strings.Split(opts.Token, ".")
	if len(split) != 2 {
		return fmt.Errorf("token must be of the form %q, but got %q", validBootstrapToken, opts.Token)
	}
	tokenID, tokenSecret := split[0], split[1]

	return tokenutils.CreateBootstrapToken(ctx, clientSet, opts.CreateOptions, tokenID, tokenSecret)
}
