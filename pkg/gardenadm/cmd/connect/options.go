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

// Flow:
// $ make kind-single-node-up gardenadm-up
// $ make gardenadm-up SCENARIO=connect
// Run `gardenadm init` on machine pod
// $ export KUBECONFIG=dev-setup/kubeconfigs/virtual-garden/kubeconfig
// $ go run ./cmd/gardenadm token create --validity=8h --print-connect-command --shoot-namespace=garden --shoot-name=root
//   ^ this command needs to write the shoot namespace/name into the description of the bootstrap token secret
//     this way, we can check in the to-be-created "shoot authorizer" if the CSR (created with the bootstrap token) has the correct gardener.cloud:system:shoot:<namespace>:<name> username
//     this prevents claiming a client certificate for another shoot
// $ gardenadm connect --bootstrap-token <token> --ca-certificate <ca-cert> <control-plane-address>

// Options contains options for this command.
type Options struct {
	*cmd.Options
	cmd.ManifestOptions

	// ControlPlaneAddress is the address of the Gardener control plane to which the autonomous shoot should be connected.
	ControlPlaneAddress string
	// BootstrapToken is the bootstrap token to use for connecting the shoot.
	BootstrapToken string
	// CertificateAuthority is the CA bundle of the control plane.
	CertificateAuthority []byte
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
	fs.StringVar(&o.BootstrapToken, "bootstrap-token", "", "Bootstrap token for connecting the autonomous shoot cluster to Gardener (create it with 'gardenadm token' in the garden cluster)")
}
