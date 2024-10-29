// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericiooptions"
)

// Options contains persistent options for all commands.
type Options struct {
	genericiooptions.IOStreams
}

// Complete completes the options.
func (o *Options) Complete() error { return nil }

// Validate validates the options.
func (o *Options) Validate() error { return nil }

func (o *Options) addFlags(_ *pflag.FlagSet) {}
