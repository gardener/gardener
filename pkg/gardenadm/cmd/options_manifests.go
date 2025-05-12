// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/spf13/pflag"
)

// ManifestOptions contains options related to handling the manifest files.
type ManifestOptions struct {
	// ConfigDir is the path to a directory containing the Gardener configuration files for the init command, i.e.,
	// files containing resources like CloudProfile, Shoot, etc.
	ConfigDir string
}

// ParseArgs parses the arguments to the options.
func (o *ManifestOptions) ParseArgs(_ []string) error { return nil }

// Validate validates the options.
func (o *ManifestOptions) Validate() error {
	if len(o.ConfigDir) == 0 {
		return fmt.Errorf("must provide a path to a config directory")
	}

	return nil
}

// Complete completes the options.
func (o *ManifestOptions) Complete() error { return nil }

// AddFlags implements Flagger.AddFlags.
func (o *ManifestOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&o.ConfigDir, "config-dir", "d", "", "Path to a directory containing "+
		"the Gardener configuration files for the init command, i.e., files containing resources like CloudProfile, "+
		"Shoot, etc. The files must be in YAML/JSON and have .{yaml,yml,json} file extensions to be considered.")
}
