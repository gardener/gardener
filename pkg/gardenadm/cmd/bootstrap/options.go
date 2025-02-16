// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	// Kubeconfig is the path to the kubeconfig file pointing to the KinD cluster.
	Kubeconfig string
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(_ []string) error {
	if o.Kubeconfig == "" {
		o.Kubeconfig = os.Getenv("KUBECONFIG")
	}

	return nil
}

// Validate validates the options.
func (o *Options) Validate() error {
	if len(o.Kubeconfig) == 0 {
		return fmt.Errorf("must provide a path to a KinD cluster kubeconfig")
	}

	return nil
}

// Complete completes the options.
func (o *Options) Complete() error { return nil }

func (o *Options) addFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&o.Kubeconfig, "kubeconfig", "k", "", "Path to the kubeconfig file pointing to the KinD cluster")
}
