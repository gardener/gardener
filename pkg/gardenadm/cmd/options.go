// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericiooptions"
)

// Options contains persistent options for all commands.
type Options struct {
	genericiooptions.IOStreams

	// Log is the global logger.
	Log logr.Logger
	// LogLevel is the log level (one of [info,debug,error]).
	LogLevel string
	// LogFormat is the log format (one of [json,text]).
	LogFormat string
}

// Complete completes the options.
func (o *Options) Complete() error { return nil }

// Validate validates the options.
func (o *Options) Validate() error {
	if !sets.New[string]("info", "debug", "error").Has(o.LogLevel) {
		return fmt.Errorf("log-level must be one of [info,debug,error]")
	}

	if !sets.New[string]("json", "text").Has(o.LogFormat) {
		return fmt.Errorf("log-format must be one of [json,text]")
	}

	return nil
}

// AddFlags adds the flags to the flag set.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&o.LogLevel, "log-level", "", "info", "The level/severity for the logs. Must be one of [info,debug,error]")
	fs.StringVarP(&o.LogFormat, "log-format", "", "text", "The format for the logs. Must be one of [json,text]")
}
