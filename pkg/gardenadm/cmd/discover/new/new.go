// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package new

import (
	"context"
	"fmt"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
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
		Use:   "new",
		Short: "Download Gardener configuration resources for a new Shoot described by a local manifest",
		Long:  "Download Gardener configuration resources (CloudProfile, ControllerRegistrations, ControllerDeployments, etc.) from an existing garden cluster for a new Shoot described by a local manifest.",
		Args:  cobra.NoArgs,

		Example: `# Download the configuration for a new Shoot described by a local manifest
gardenadm discover new --manifest <path-to-shoot-manifest>`,

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

	shoot, err := readShootManifest(fs, opts.Manifest)
	if err != nil {
		return fmt.Errorf("failed reading shoot manifest from %q: %w", opts.Manifest, err)
	}

	return shared.RunForShoot(ctx, opts.CommonOptions, clientSet.Client(), fs, shoot, false)
}

var (
	versions = schema.GroupVersions([]schema.GroupVersion{gardencorev1.SchemeGroupVersion, gardencorev1beta1.SchemeGroupVersion})
	decoder  = kubernetes.GardenCodec.CodecForVersions(kubernetes.GardenSerializer, kubernetes.GardenSerializer, versions, versions)
)

func readShootManifest(fs afero.Afero, manifestPath string) (*gardencorev1beta1.Shoot, error) {
	shootManifest, err := fs.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	shoot := &gardencorev1beta1.Shoot{}
	if _, _, err := decoder.Decode(shootManifest, nil, shoot); err != nil {
		return nil, err
	}

	return shoot, nil
}
