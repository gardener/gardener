// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package existing

import (
	"context"
	"fmt"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	"github.com/gardener/gardener/pkg/gardenadm/cmd/discover/internal/shared"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{CommonOptions: &shared.CommonOptions{Options: globalOpts}}

	cmd := &cobra.Command{
		Use:   "existing",
		Short: "Download Gardener configuration resources for an existing Shoot in the garden cluster",
		Long:  "Download Gardener configuration resources (CloudProfile, ControllerRegistrations, ControllerDeployments, etc.) from an existing garden cluster for an existing Shoot.",
		Args:  cobra.NoArgs,

		Example: `# Download the configuration for an existing Shoot
gardenadm discover existing --name <name> --namespace <namespace>`,

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

var (
	// NewClientSetFromFile is an alias for botanist.NewClientSetFromFile.
	// Exposed for unit testing.
	NewClientSetFromFile = botanist.NewClientSetFromFile
	// NewAferoFs is an alias for returning an afero.NewOsFs.
	// Exposed for unit testing.
	NewAferoFs = func() afero.Afero { return afero.Afero{Fs: afero.NewOsFs()} }
)

func run(ctx context.Context, opts *Options) error {
	fs := NewAferoFs()

	clientSet, err := NewClientSetFromFile(opts.Kubeconfig, kubernetes.GardenScheme)
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}

	shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: opts.Name, Namespace: opts.Namespace}}
	if err := clientSet.Client().Get(ctx, client.ObjectKeyFromObject(shoot), shoot); err != nil {
		return fmt.Errorf("failed getting Shoot %s from garden cluster: %w", client.ObjectKeyFromObject(shoot), err)
	}

	return shared.RunForShoot(ctx, opts.CommonOptions, clientSet.Client(), fs, shoot, true)
}
