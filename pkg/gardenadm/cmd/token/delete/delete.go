// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package delete

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	tokenutils "github.com/gardener/gardener/pkg/gardenadm/cmd/token/utils"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "delete [token-values...]",
		Short: "Delete one or more bootstrap tokens from the cluster",
		Long: `Delete one or more bootstrap tokens from the cluster.

The [token-value] is the ID of the token of the form "[a-z0-9]{6}" to delete.
Alternatively, it can be the full token value of the form "[a-z0-9]{6}.[a-z0-9]{16}".
A third option is to specify the name of the Secret in the form "bootstrap-token-[a-z0-9]{6}".

You can delete multiple tokens by providing multiple token values separated by spaces.`,

		Example: `# Delete a single bootstrap token with ID "foo123" from the cluster
gardenadm token delete foo123

# Delete multiple bootstrap tokens with IDs "foo123", "bar456", and "789baz" from the cluster
gardenadm token delete foo123 bootstrap-token-bar456 789baz.abcdef1234567890

# Attempt to delete a token that does not exist (will not throw an error if the token is already deleted)
gardenadm token delete nonexisting123`,

		Args: cobra.MinimumNArgs(1),

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

	for _, tokenID := range opts.TokenIDs {
		if err := clientSet.Client().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: bootstraptokenutil.BootstrapTokenSecretName(tokenID), Namespace: metav1.NamespaceSystem}}); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed deleting bootstrap token secret with ID %q: %w", tokenID, err)
		}

		fmt.Fprintf(opts.Out, "bootstrap token %q deleted\n", tokenID)
	}

	return nil
}
