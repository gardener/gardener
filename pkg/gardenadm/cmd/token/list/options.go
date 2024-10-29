// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package list

import (
	"github.com/spf13/pflag"
)

// Options contains options for this command.
type Options struct{}

// Complete completes the options.
func (o *Options) Complete() error { return nil }

// Validate validates the options.
func (o *Options) Validate() error { return nil }

func (o *Options) addFlags(_ *pflag.FlagSet) {}
