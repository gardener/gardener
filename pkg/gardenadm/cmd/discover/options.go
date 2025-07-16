// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	cmd.ManifestOptions

	// Kubeconfig is the path to the kubeconfig file pointing to the garden cluster.
	Kubeconfig string
	// ShootManifest is the path to the shoot manifest file.
	ShootManifest string
	// RunsControlPlane indicates whether the control plane is run in the same cluster. This should be set to false
	// if `gardenadm bootstrap` will be used for bootstrapping the shoot cluster.
	RunsControlPlane bool
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(args []string) error {
	if o.Kubeconfig == "" {
		o.Kubeconfig = os.Getenv("KUBECONFIG")
	}
	if len(args) > 0 {
		o.ShootManifest = strings.TrimSpace(args[0])

		if len(o.ConfigDir) == 0 {
			o.ConfigDir = filepath.Dir(o.ShootManifest)
		}
	}

	return o.ManifestOptions.ParseArgs(args)
}

// Validate validates the options.
func (o *Options) Validate() error {
	if len(o.Kubeconfig) == 0 {
		return fmt.Errorf("must provide a path to a garden cluster kubeconfig")
	}
	if len(o.ShootManifest) == 0 {
		return fmt.Errorf("must provide a path to the shoot manifest file")
	}

	return o.ManifestOptions.Validate()
}

// Complete completes the options.
func (o *Options) Complete() error { return o.ManifestOptions.Complete() }

func (o *Options) addFlags(fs *pflag.FlagSet) {
	o.ManifestOptions.AddFlags(fs)
	fs.StringVarP(&o.Kubeconfig, "kubeconfig", "k", "", "Path to the kubeconfig file pointing to the garden cluster")
	fs.BoolVar(&o.RunsControlPlane, "runs-control-plane", true, "Indicates whether the control plane is run in the same cluster. This should be set to false if `gardenadm bootstrap` will be used for bootstrapping the shoot cluster.")
}
