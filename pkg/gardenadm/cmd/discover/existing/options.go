// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package existing

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd/discover/internal/shared"
)

// Options contains options for the `gardenadm discover existing` command.
type Options struct {
	*shared.CommonOptions

	// Name is the name of an existing Shoot in the garden cluster to discover resources for.
	Name string
	// Namespace is the namespace of an existing Shoot in the garden cluster to discover resources for.
	Namespace string
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(args []string) error {
	return o.CommonOptions.ParseArgs(args)
}

// Validate validates the options.
func (o *Options) Validate() error {
	if len(o.Name) == 0 {
		return fmt.Errorf("must provide --name")
	}
	if len(o.Namespace) == 0 {
		return fmt.Errorf("must provide --namespace")
	}

	return o.CommonOptions.Validate()
}

// Complete completes the options.
func (o *Options) Complete() error {
	return o.CommonOptions.Complete()
}

func (o *Options) addFlags(fs *pflag.FlagSet) {
	o.AddFlags(fs)
	fs.StringVar(&o.Name, "name", "", "Name of an existing Shoot in the garden cluster to discover resources for.")
	fs.StringVar(&o.Namespace, "namespace", "", "Namespace of an existing Shoot in the garden cluster to discover resources for.")
}
