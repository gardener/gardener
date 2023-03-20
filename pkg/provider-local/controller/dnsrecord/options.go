// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dnsrecord

import (
	"github.com/spf13/pflag"

	"github.com/gardener/gardener/extensions/pkg/controller/cmd"
)

// ControllerOptions are command line options that can be set for controller.Options.
type ControllerOptions struct {
	// MaxConcurrentReconciles are the maximum concurrent reconciles.
	MaxConcurrentReconciles int
	// WriteToHostsFile specifies whether to enable hosts management via /etc/hosts.
	WriteToHostsFile bool

	config *ControllerConfig
}

// AddFlags implements Flagger.AddFlags.
func (c *ControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&c.MaxConcurrentReconciles, cmd.MaxConcurrentReconcilesFlag, c.MaxConcurrentReconciles, "The maximum number of concurrent reconciliations.")
	fs.BoolVar(&c.WriteToHostsFile, "write-to-hosts-file", true, "Whether to enable hosts management via /etc/hosts.")
}

// Complete implements Completer.Complete.
func (c *ControllerOptions) Complete() error {
	c.config = &ControllerConfig{c.MaxConcurrentReconciles, c.WriteToHostsFile}
	return nil
}

// Completed returns the completed ControllerConfig. Only call this if `Complete` was successful.
func (c *ControllerOptions) Completed() *ControllerConfig {
	return c.config
}

// ControllerConfig is a completed controller configuration.
type ControllerConfig struct {
	// MaxConcurrentReconciles is the maximum number of concurrent reconciles.
	MaxConcurrentReconciles int
	// WriteToHostsFile specifies whether to enable hosts management via /etc/hosts.
	WriteToHostsFile bool
}

// Apply sets the values of this ControllerConfig in the given AddOptions.
func (c *ControllerConfig) Apply(opts *AddOptions) {
	opts.Controller.MaxConcurrentReconciles = c.MaxConcurrentReconciles
	opts.WriteToHostsFile = c.WriteToHostsFile
}
