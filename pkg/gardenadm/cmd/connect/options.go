// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package connect

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	cmd.ManifestOptions

	// ControlPlaneAddress is the address of the Gardener control plane to which the self-hosted shoot should be connected.
	ControlPlaneAddress string
	// BootstrapToken is the bootstrap token to use for connecting the shoot.
	BootstrapToken string
	// CertificateAuthority is the CA bundle of the control plane.
	CertificateAuthority []byte
	// Force forces the deployment of gardenlet, even if it already exists.
	Force bool
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(args []string) error {
	if len(args) > 0 {
		o.ControlPlaneAddress = strings.TrimSpace(args[0])
	}

	return o.ManifestOptions.ParseArgs(args)
}

// Validate validates the options.
func (o *Options) Validate() error {
	if len(o.BootstrapToken) == 0 {
		return fmt.Errorf("must provide a bootstrap token")
	}

	if len(o.ConfigDir) == 0 {
		// `gardenadm init` stores the path of the config directory in the cmd.ConfigDirLocation file on the machine's
		// file system. Hence, we can default it to this location if the user does not explicitly provide us with the
		// config directory.
		data, err := os.ReadFile(cmd.ConfigDirLocation)
		if err != nil {
			return fmt.Errorf("error reading config dir location file %s: %w", cmd.ConfigDirLocation, err)
		}
		o.ConfigDir = string(data)
	}

	return o.ManifestOptions.Validate()
}

// Complete completes the options.
func (o *Options) Complete() error { return o.ManifestOptions.Complete() }

func (o *Options) addFlags(fs *pflag.FlagSet) {
	o.ManifestOptions.AddFlags(fs)
	fs.BytesBase64Var(&o.CertificateAuthority, "ca-certificate", nil, "Base64-encoded certificate authority bundle of the Gardener control plane")
	fs.StringVar(&o.BootstrapToken, "bootstrap-token", "", "Bootstrap token for connecting the self-hosted shoot cluster to a garden cluster (create it with 'gardenadm token' in the garden cluster)")
	fs.BoolVar(&o.Force, "force", false, "Forces the deployment of gardenlet, even if it already exists")
}
