// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package destroy

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	cmd.ManifestOptions

	// Kubeconfig is the path to the kubeconfig file pointing to the bootstrap cluster.
	Kubeconfig string
	// ShootKubeconfig is the path to the kubeconfig file pointing to the self-hosted shoot cluster. If set, the ShootState
	// is computed from the shoot cluster instead of being read from the config directory.
	ShootKubeconfig string
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(args []string) error {
	if err := cmd.DefaultKubeconfig(&o.Kubeconfig); err != nil {
		return fmt.Errorf("could not default kubeconfig: %w", err)
	}

	return o.ManifestOptions.ParseArgs(args)
}

// Validate validates the options.
func (o *Options) Validate() error {
	if len(o.Kubeconfig) == 0 {
		return fmt.Errorf("must provide a path to a bootstrap cluster kubeconfig")
	}

	return o.ManifestOptions.Validate()
}

// Complete completes the options.
func (o *Options) Complete() error {
	return o.ManifestOptions.Complete()
}

func (o *Options) addFlags(fs *pflag.FlagSet) {
	o.ManifestOptions.AddFlags(fs)
	fs.StringVarP(&o.Kubeconfig, "kubeconfig", "k", "", "Path to the kubeconfig file pointing to the bootstrap cluster")
	fs.StringVar(&o.ShootKubeconfig, "shoot-kubeconfig", "", "Path to the kubeconfig file pointing to the self-hosted shoot cluster (recommended). If set, cluster-autoscaler and machine-controller-manager are stopped in the shoot cluster, and the current ShootState is computed from the shoot cluster instead of being read from the config directory")
}
