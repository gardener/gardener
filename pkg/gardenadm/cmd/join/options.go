// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package join

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	// ControlPlaneAddress is the address of the control plane to which the node should be joined.
	ControlPlaneAddress string
	// BootstrapToken is the bootstrap token to use for joining the node.
	BootstrapToken string
	// CertificateAuthority is the CA bundle of the control plane.
	CertificateAuthority []byte
	// GardenerNodeAgentSecretName is the name of the secret from which gardener-node-agent should download its
	// operating system configuration.
	GardenerNodeAgentSecretName string
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(args []string) error {
	if len(args) > 0 {
		o.ControlPlaneAddress = strings.TrimSpace(args[0])
	}

	return nil
}

// Validate validates the options.
func (o *Options) Validate() error {
	if len(o.BootstrapToken) == 0 {
		return fmt.Errorf("must provide a bootstrap token")
	}
	if len(o.GardenerNodeAgentSecretName) == 0 {
		return fmt.Errorf("must provide a secret name for gardener-node-agent")
	}

	return nil
}

// Complete completes the options.
func (o *Options) Complete() error { return nil }

func (o *Options) addFlags(fs *pflag.FlagSet) {
	fs.BytesBase64Var(&o.CertificateAuthority, "ca-certificate", nil, "Base64-encoded certificate authority bundle of the control plane")
	fs.StringVar(&o.BootstrapToken, "bootstrap-token", "", "Bootstrap token for joining the cluster (create it with gardenadm token)")
	fs.StringVar(&o.GardenerNodeAgentSecretName, "gardener-node-agent-secret-name", "", "Name of the Secret from which gardener-node-agent should download its operating system configuration")
}
