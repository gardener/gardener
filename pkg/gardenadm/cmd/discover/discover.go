// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discover

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
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Conveniently download Gardener configuration resources from an existing garden cluster",
		Long:  "Conveniently download Gardener configuration resources from an existing garden cluster (CloudProfile, ControllerRegistrations, ControllerDeployments, etc.)",

		Example: `# Download the configuration
gardenadm discover <path-to-shoot-manifest>`,

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
	// NewClientSetFromFile in alias for botanist.NewClientSetFromFile.
	// Exposed for unit testing.
	NewClientSetFromFile = botanist.NewClientSetFromFile
	// NewAferoFs in alias for afero.NewOsFs.
	// Exposed for unit testing.
	NewAferoFs = afero.NewOsFs
)

func run(_ context.Context, opts *Options) error {
	fs := afero.Afero{Fs: NewAferoFs()}

	shoot, err := readShoot(fs, opts.ShootManifest)
	if err != nil {
		return fmt.Errorf("failed reading shoot manifest from %q: %w", opts.ShootManifest, err)
	}

	clientSet, err := NewClientSetFromFile(opts.Kubeconfig, kubernetes.GardenScheme)
	if err != nil {
		return fmt.Errorf("failed creating client: %w", err)
	}

	_ = clientSet
	_ = shoot

	return nil
}

var (
	versions = schema.GroupVersions([]schema.GroupVersion{gardencorev1.SchemeGroupVersion, gardencorev1beta1.SchemeGroupVersion})
	decoder  = kubernetes.GardenCodec.CodecForVersions(kubernetes.GardenSerializer, kubernetes.GardenSerializer, versions, versions)
)

func readShoot(fs afero.Afero, manifestPath string) (*gardencorev1beta1.Shoot, error) {
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
