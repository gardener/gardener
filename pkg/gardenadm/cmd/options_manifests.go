// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenadm"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
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

// DirFS returns an fs.FS for the files in the given directory.
// Exposed for testing.
var DirFS = os.DirFS

// NewAutonomousBotanist reads the manifests from ConfigDir and initializes a new AutonomousBotanist with them.
func (o *ManifestOptions) NewAutonomousBotanist(ctx context.Context, log logr.Logger, clientSet kubernetes.Interface) (*botanist.AutonomousBotanist, error) {
	cloudProfile, project, shoot, controllerRegistrations, controllerDeployments, err := gardenadm.ReadManifests(log, DirFS(o.ConfigDir))
	if err != nil {
		return nil, fmt.Errorf("failed reading Kubernetes resources from config directory %s: %w", o.ConfigDir, err)
	}

	extensions, err := botanist.ComputeExtensions(shoot, controllerRegistrations, controllerDeployments)
	if err != nil {
		return nil, fmt.Errorf("failed computing extensions: %w", err)
	}

	b, err := botanist.NewAutonomousBotanist(ctx, log, clientSet, project, cloudProfile, shoot, extensions)
	if err != nil {
		return nil, fmt.Errorf("failed constructing botanist: %w", err)
	}

	return b, nil
}
