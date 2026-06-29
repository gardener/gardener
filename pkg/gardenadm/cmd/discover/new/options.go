// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package new

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd/discover/internal/shared"
)

// Options contains options for the `gardenadm discover new` command.
type Options struct {
	*shared.CommonOptions

	// Manifest is the path to the Shoot manifest file describing a new Shoot to discover resources for.
	Manifest string
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(args []string) error {
	return o.CommonOptions.ParseArgs(args)
}

// Validate validates the options.
func (o *Options) Validate() error {
	if len(o.Manifest) == 0 {
		return fmt.Errorf("must provide --manifest")
	}

	// Default the config dir from the Shoot manifest path before validating ManifestOptions, so users do not have to
	// pass --config-dir when --manifest already points inside the desired output directory.
	if len(o.ConfigDir) == 0 {
		o.ConfigDir = filepath.Dir(o.Manifest)
	}

	return o.CommonOptions.Validate()
}

// Complete completes the options.
func (o *Options) Complete() error {
	return o.CommonOptions.Complete()
}

func (o *Options) addFlags(fs *pflag.FlagSet) {
	o.AddFlags(fs)
	fs.StringVar(&o.Manifest, "manifest", "", "Path to a Shoot manifest file describing a new Shoot to discover resources for.")
}
