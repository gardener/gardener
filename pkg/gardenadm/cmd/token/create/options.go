// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package create

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

// Options contains options for this command.
type Options struct {
	// Token is the token to create.
	Token string
}

// Complete completes the options.
func (o *Options) Complete(args []string) error {
	if len(args) > 0 {
		o.Token = strings.TrimSpace(args[0])
	}

	if o.Token == "" {
		// TODO: Generate a random token instead.
		o.Token = "foo123.bar4567890baz123"
	}

	return nil
}

// Validate validates the options.
func (o *Options) Validate() error {
	if o.Token == "" {
		return fmt.Errorf("must provide a token to create")
	}

	// TODO: Validate that the token has the correct/expected format

	return nil
}

func (o *Options) addFlags(_ *pflag.FlagSet) {}
