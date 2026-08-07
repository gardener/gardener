// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reset

import (
	"fmt"
	"strings"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/drain"
	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options

	// ControlPlaneAddress is the address of the control plane from which the node should be removed.
	ControlPlaneAddress string
	// Token is the token to use for resetting the node.
	Token string
	// CertificateAuthority is the CA bundle of the control plane.
	CertificateAuthority []byte
	// Timeout is the timeout for the node drain.
	Timeout time.Duration
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
	if len(o.Token) == 0 {
		return fmt.Errorf("must provide a token")
	}

	if o.Timeout < 0 {
		return fmt.Errorf("must provide a timeout >= 0")
	}

	return nil
}

// Complete completes the options.
func (o *Options) Complete() error { return nil }

func (o *Options) addFlags(fs *pflag.FlagSet) {
	fs.BytesBase64Var(&o.CertificateAuthority, "ca-certificate", nil, "Base64-encoded certificate authority bundle of the control plane")
	fs.StringVar(&o.Token, "token", "", "Token for removing the node from the cluster (create it with 'gardenadm token' on a control plane node)")
	fs.DurationVar(&o.Timeout, "timeout", drain.DefaultMachineDrainTimeout, "Timeout for draining the node")
}
