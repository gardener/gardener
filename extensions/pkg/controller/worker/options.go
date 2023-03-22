// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
