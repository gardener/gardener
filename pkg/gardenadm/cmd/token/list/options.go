// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package list

import (
	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	// WithTokenSecret specifies whether the token secret should be displayed.
	WithTokenSecret bool
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(_ []string) error { return nil }

// Validate validates the options.
func (o *Options) Validate() error { return nil }

// Complete completes the options.
func (o *Options) Complete() error { return nil }

func (o *Options) addFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.WithTokenSecret, "with-token-secret", false, "Display the token secret")
}
