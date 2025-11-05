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
	// WorkerPoolName is the name of the worker pool to use for the join command. If not provided, the node is assigned
	// to the first worker pool in the Shoot manifest.
	WorkerPoolName string
	// ControlPlane indicates whether the node should be joined as a control plane node.
	ControlPlane bool
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

	if o.ControlPlane && o.WorkerPoolName != "" {
		return fmt.Errorf("cannot provide a worker pool name when joining a control plane node")
	}

	return nil
}

// Complete completes the options.
func (o *Options) Complete() error { return nil }

func (o *Options) addFlags(fs *pflag.FlagSet) {
	fs.BytesBase64Var(&o.CertificateAuthority, "ca-certificate", nil, "Base64-encoded certificate authority bundle of the control plane")
	fs.StringVar(&o.BootstrapToken, "bootstrap-token", "", "Bootstrap token for joining the cluster (create it with 'gardenadm token' on a control plane node)")
	fs.StringVarP(&o.WorkerPoolName, "worker-pool-name", "w", "", "Name of the worker pool to assign the joining node.")
	fs.BoolVar(&o.ControlPlane, "control-plane", false, "Create a new control plane instance on this node")
}
