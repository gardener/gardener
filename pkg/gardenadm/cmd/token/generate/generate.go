// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	tokenutils "github.com/gardener/gardener/pkg/gardenadm/cmd/token/utils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{
		Options:       globalOpts,
		CreateOptions: &tokenutils.Options{Options: globalOpts},
	}

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a random bootstrap token for joining a node",
		Long: `Generate a random bootstrap token that can be used for joining a node to an autonomous shoot cluster.
Note that the token is not created on the server (use 'gardenadm token create' for it).
The token is securely generated and follows the format "[a-z0-9]{6}.[a-z0-9]{16}".
Read more about it here: https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/`,

		Example: `# Generate a random bootstrap token for joining a node
gardenadm token generate`,

		Args: cobra.ExactArgs(0),

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

	tokenID, err := utils.GenerateRandomStringFromCharset(6, bootstraptoken.CharSet)
	if err != nil {
		return fmt.Errorf("failed computing random token ID: %w", err)
	}

	return tokenutils.CreateBootstrapToken(ctx, clientSet, opts.CreateOptions, tokenID, "")
}
