// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	tokenutils "github.com/gardener/gardener/pkg/gardenadm/cmd/token/utils"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	// CreateOptions are the options for creating a bootstrap token.
	CreateOptions *tokenutils.Options
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(args []string) error {
	return o.CreateOptions.ParseArgs(args)
}

// Validate validates the options.
func (o *Options) Validate() error {
	return o.CreateOptions.Validate()
}

// Complete completes the options.
func (o *Options) Complete() error {
	return o.CreateOptions.Complete()
}

func (o *Options) addFlags(fs *pflag.FlagSet) {
	o.CreateOptions.AddFlags(fs)
}
