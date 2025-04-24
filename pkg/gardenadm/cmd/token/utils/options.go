// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/pkg/gardenadm/cmd"
)

// Options contains options for this command.
type Options struct {
	*cmd.Options
	// Description is the description for the bootstrap token.
	Description string
	// Validity duration of the bootstrap token.
	Validity time.Duration
}

// ParseArgs parses the arguments to the options.
func (o *Options) ParseArgs(_ []string) error { return nil }

// Validate validates the options.
func (o *Options) Validate() error {
	if minValidity := 10 * time.Minute; o.Validity < minValidity {
		return fmt.Errorf("minimum validity duration is %s", minValidity)
	}
	if maxValidity := 24 * time.Hour; o.Validity > maxValidity {
		return fmt.Errorf("maximum validity duration is %s", maxValidity)
	}

	return nil
}

// Complete completes the options.
func (o *Options) Complete() error { return nil }

// AddFlags adds the flags to the command line flag set.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&o.Description, "description", "d", "Used for joining nodes via `gardenadm join`", "Description for the bootstrap token")
	fs.DurationVarP(&o.Validity, "validity", "v", time.Hour, "Validity duration of the bootstrap token")
}
