// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"github.com/spf13/pflag"
)

const (
	// DeployCRDsFlag is the name of the command line flag to specify whether the worker CRDs
	// should be deployed or not.
	DeployCRDsFlag = "deploy-crds"
)

// Options are command line options that can be set for controller.Options.
type Options struct {
	// DeployCRDs defines whether to ignore the operation annotation or not.
	DeployCRDs bool

	config *Config
}

// AddFlags implements Flagger.AddFlags.
func (c *Options) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&c.DeployCRDs, DeployCRDsFlag, c.DeployCRDs, "Deploy the required worker CRDs.")
}

// Complete implements Completer.Complete.
func (c *Options) Complete() error {
	c.config = &Config{c.DeployCRDs}
	return nil
}

// Completed returns the completed Config. Only call this if `Complete` was successful.
func (c *Options) Completed() *Config {
	return c.config
}

// Config is a completed controller configuration.
type Config struct {
	// DeployCRDs defines whether to ignore the operation annotation or not.
	DeployCRDs bool
}

// Apply sets the values of this Config in the given controller.Options.
func (c *Config) Apply(ignore *bool) {
	*ignore = c.DeployCRDs
}
