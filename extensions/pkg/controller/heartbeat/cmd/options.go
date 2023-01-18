// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cmd

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/gardener/gardener/extensions/pkg/controller/heartbeat"
)

// Options are command line options that can be set for the heartbeat controller.
type Options struct {
	ExtensionName string
	// Namespace is the namespace which will be used for the heartbeat lease resource.
	Namespace string
	// RenewIntervalSeconds defines how often the heartbeat lease is renewed.
	RenewIntervalSeconds int32

	config *Config
}

// AddFlags implements Flagger.AddFlags.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.Namespace, "namespace", o.Namespace, "The namespace to use for the heartbeat lease resource.")
	fs.Int32Var(&o.RenewIntervalSeconds, "renew-interval-seconds", 30, "How often the heartbeat lease will be renewed. Default is 30 seconds.")
}

// Validate validates the options.
func (o *Options) Validate() error {
	if o.RenewIntervalSeconds <= 0 {
		return fmt.Errorf("--heartbeat-renew-interval-seconds must be greater than 0")
	}
	return nil
}

// Complete implements Completer.Complete.
func (o *Options) Complete() error {
	o.config = &Config{
		ExtensionName:        o.ExtensionName,
		Namespace:            o.Namespace,
		RenewIntervalSeconds: o.RenewIntervalSeconds,
	}
	return nil
}

// Completed returns the completed Config. Only call this if `Complete` was successful.
func (o *Options) Completed() *Config {
	return o.config
}

// Config is a completed heartbeat controller configuration.
type Config struct {
	ExtensionName string
	// Namespace is the namespace which will be used for heartbeat lease resource.
	Namespace string
	// RenewIntervalSeconds defines how often the heartbeat lease is renewed.
	RenewIntervalSeconds int32
}

// Apply sets the values of this Config in the given heartbeat.AddOptions.
func (c *Config) Apply(opts *heartbeat.AddOptions) {
	opts.ExtensionName = c.ExtensionName
	opts.Namespace = c.Namespace
	opts.RenewIntervalSeconds = c.RenewIntervalSeconds
}
