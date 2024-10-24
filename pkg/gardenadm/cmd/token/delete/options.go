// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package delete

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

// Options contains options for this command.
type Options struct {
	// TokenID is the ID of the token to delete.
	TokenID string
}

// Complete completes the options.
func (o *Options) Complete(args []string) error {
	if len(args) > 0 {
		o.TokenID = strings.TrimSpace(args[0])
	}

	return nil
}

// Validate validates the options.
func (o *Options) Validate() error {
	if o.TokenID == "" {
		return fmt.Errorf("must provide a token ID to delete")
	}

	return nil
}

func (o *Options) addFlags(_ *pflag.FlagSet) {}
