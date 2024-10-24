// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"fmt"

	"github.com/spf13/pflag"
)

// Options contains options for this command.
type Options struct {
	// Kubeconfig is the path to the kubeconfig file pointing to the KinD cluster.
	Kubeconfig string
}

// Complete completes the options.
func (o *Options) Complete() error { return nil }

// Validate validates the options.
func (o *Options) Validate() error {
	if len(o.Kubeconfig) == 0 {
		return fmt.Errorf("must provide a path to a KinD cluster kubeconfig")
	}

	return nil
}

func (o *Options) addFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&o.Kubeconfig, "kubeconfig", "k", "", "Path to the kubeconfig file pointing to the KinD cluster")
}
